package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"unstablestress/backend/internal/arguments"
	"unstablestress/backend/internal/cli"
	"unstablestress/backend/internal/protocol"
)

const (
	fetchPath      = "/fetch"
	rconPath       = "/rcon"
	defaultJSONDir = "配置"
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
		logger.Fatalf("配置不可用：%v", err)
	}

	prober, err := arguments.NewProber(config.probeTimeout)
	if err != nil {
		logger.Fatalf("配置不可用：%v", err)
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
		logger.Printf("REST API 已就绪 addr=%s users=%d methods=%d fetch=%s rcon=%s",
			config.address, len(store.Users), len(store.Methods), fetchPath, rconPath)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatalf("服务器意外停止：%v", err)
		}
	}()

	<-ctx.Done()
	logger.Printf("收到退出信号，等待 %s 完成收尾", shutdownWindow)

	drainCtx, cancel := context.WithTimeout(context.Background(), shutdownWindow)
	defer cancel()

	if err := server.Shutdown(drainCtx); err != nil {
		logger.Printf("优雅退出未完成：%v", err)
	}
	logger.Printf("已停止")
}

func parseFlags() settings {
	var config settings

	set := cli.New("api")
	set.StringVar(&config.address, "addr", ":8080", "REST API 监听的地址")
	set.StringVar(&config.jsonDir, "json-dir", defaultJSONDir, "存放 users.json、methods.json 和 blacklist.json 的目录")
	set.StringVar(&config.botToken, "bot-token", os.Getenv("BOT_TOKEN"), "机器人必须在 "+protocol.BotTokenHeader+" 标头中发送的共享密钥")
	set.StringVar(&config.probeTimeout, "probe-timeout", probeTimeout, "检测主机是否提供 HTTPS 服务时使用的超时时间")
	set.DurationVar(&config.reapInterval, "slot-reap-interval", time.Second, "回收过期槽位（slot）的频率")
	set.Usage = func() { cli.Usage(set, os.Stderr) }

	if err := cli.Parse(set, os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			cli.Usage(set, os.Stdout)
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, err)
		set.Usage()
		os.Exit(2)
	}

	return config
}

func newHTTPServer(address string, fetch *fetchServer, logger *log.Logger) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc(fetchPath, fetch.handle)
	mux.HandleFunc(rconPath, fetch.hub.handle)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		logger.Printf("已拒绝来自 %s 对未知路径 %s 的请求", r.RemoteAddr, r.URL.Path)
		writeProblem(w, http.StatusNotFound, "未知端点 %s", r.URL.Path)
	})

	return &http.Server{
		Addr:              address,
		Handler:           mux,
		ReadHeaderTimeout: headerTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
}
