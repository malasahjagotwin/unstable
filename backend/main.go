package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"unstablestress/backend/arguments"
	"unstablestress/backend/internal/protocol"
)

const (
	fetchPath      = "/fetch"
	rconPath       = "/rcon"
	shutdownWindow = 10 * time.Second
	headerTimeout  = 10 * time.Second
	writeTimeout   = 30 * time.Second
	idleTimeout    = 2 * time.Minute
	probeTimeout   = "3s"
)

type settings struct {
	address      string
	jsonDir      string
	botToken     string
	probeTimeout string
	reapInterval time.Duration
}

func main() {
	config := parseFlags()
	logger := log.New(os.Stdout, "", log.LstdFlags|log.Lmsgprefix)

	store, err := LoadStore(config.jsonDir)
	if err != nil {
		logger.Fatalf("configuration is unusable: %v", err)
	}

	prober, err := arguments.NewProber(config.probeTimeout)
	if err != nil {
		logger.Fatalf("configuration is unusable: %v", err)
	}

	slots := newSlotManager()
	botHub := newHub(config.botToken, logger)
	fetch := &fetchServer{store: store, slots: slots, hub: botHub, prober: prober, logger: logger}

	stopSlots := make(chan struct{})
	defer close(stopSlots)
	go slots.reapLoop(stopSlots, config.reapInterval)

	server := newHTTPServer(config.address, fetch, logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Printf("REST API ready addr=%s users=%d methods=%d fetch=%s rcon=%s",
			config.address, len(store.Users), len(store.Methods), fetchPath, rconPath)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatalf("server stopped unexpectedly: %v", err)
		}
	}()

	<-ctx.Done()
	logger.Printf("shutdown signal received, draining for %s", shutdownWindow)

	drainCtx, cancel := context.WithTimeout(context.Background(), shutdownWindow)
	defer cancel()

	if err := server.Shutdown(drainCtx); err != nil {
		logger.Printf("graceful shutdown did not complete: %v", err)
	}
	logger.Printf("stopped")
}

func parseFlags() settings {
	var config settings

	flag.StringVar(&config.address, "addr", ":8080", "address the REST API listens on")
	flag.StringVar(&config.jsonDir, "json-dir", "json", "directory holding users.json, methods.json and blacklist.json")
	flag.StringVar(&config.botToken, "bot-token", os.Getenv("BOT_TOKEN"), "shared secret bots must send in the "+protocol.BotTokenHeader+" header")
	flag.StringVar(&config.probeTimeout, "probe-timeout", probeTimeout, "timeout used when detecting whether a host serves HTTPS")
	flag.DurationVar(&config.reapInterval, "slot-reap-interval", time.Second, "how often expired slots are reclaimed")
	flag.Parse()

	return config
}

func newHTTPServer(address string, fetch *fetchServer, logger *log.Logger) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc(fetchPath, fetch.handle)
	mux.HandleFunc(rconPath, fetch.hub.handle)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		logger.Printf("rejected request to unknown path %s from %s", r.URL.Path, r.RemoteAddr)
		writeProblem(w, http.StatusNotFound, "unknown endpoint %s", r.URL.Path)
	})

	return &http.Server{
		Addr:              address,
		Handler:           mux,
		ReadHeaderTimeout: headerTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
}
