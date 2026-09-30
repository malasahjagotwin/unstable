package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"unstablestress/backend/internal/protocol"
)

const (
	defaultEndpoint    = "ws://127.0.0.1:8080/rcon"
	defaultReconnect   = 2 * time.Second
	maxReconnect       = 30 * time.Second
	defaultGrace       = 15 * time.Second
	defaultConcurrency = 64
	handshakeTimeout   = 10 * time.Second
	readMessageLimit   = 1 << 20
	writeReportWait    = 10 * time.Second
)

type settings struct {
	endpoint    string
	token       string
	workDir     string
	reconnect   time.Duration
	grace       time.Duration
	concurrency int
}

type reporter struct {
	mutex sync.Mutex
	conn  *websocket.Conn
}

func main() {
	config := parseFlags()
	logger := log.New(os.Stdout, "", log.LstdFlags|log.Lmsgprefix)

	logger.Printf("bot starting endpoint=%s work_dir=%s max_concurrent=%d", config.endpoint, config.workDir, config.concurrency)

	backoff := config.reconnect
	for {
		err := serve(config, logger)
		if err != nil {
			logger.Printf("link to %s failed: %v", config.endpoint, err)
		}

		logger.Printf("retrying in %s", backoff)
		time.Sleep(backoff)
		backoff = nextBackoff(backoff)
	}
}

func parseFlags() settings {
	var config settings

	flag.StringVar(&config.endpoint, "endpoint", defaultEndpoint, "REST API rcon WebSocket endpoint")
	flag.StringVar(&config.token, "token", os.Getenv("BOT_TOKEN"), "shared secret sent in the "+protocol.BotTokenHeader+" header")
	flag.StringVar(&config.workDir, "work-dir", executableDir(), "directory used to resolve relative commands and to hold their output")
	flag.DurationVar(&config.reconnect, "reconnect", defaultReconnect, "initial delay before reconnecting")
	flag.DurationVar(&config.grace, "grace", defaultGrace, "extra time granted on top of the task duration before killing the process")
	flag.IntVar(&config.concurrency, "max-concurrent", defaultConcurrency, "largest number of tasks running at the same time")
	flag.Parse()

	return config
}

func executableDir() string {
	executable, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(executable)
}

func nextBackoff(current time.Duration) time.Duration {
	doubled := current * 2
	if doubled > maxReconnect {
		return maxReconnect
	}
	return doubled
}

func serve(config settings, logger *log.Logger) error {
	dialer := websocket.Dialer{HandshakeTimeout: handshakeTimeout}

	headers := http.Header{}
	if config.token != "" {
		headers.Set(protocol.BotTokenHeader, config.token)
	}

	conn, response, err := dialer.Dial(config.endpoint, headers)
	if err != nil {
		return fmt.Errorf("dial: %w", describeDialFailure(response, err))
	}
	defer conn.Close()

	conn.SetReadLimit(readMessageLimit)
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(2 * handshakeTimeout))
	})

	out := &reporter{conn: conn}
	slots := make(chan struct{}, config.concurrency)

	logger.Printf("connected to %s", config.endpoint)

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}

		var task protocol.Task
		if err := json.Unmarshal(data, &task); err != nil {
			logger.Printf("discarding an unreadable task: %v", err)
			continue
		}

		slots <- struct{}{}
		go func(job protocol.Task) {
			defer func() { <-slots }()
			result := run(job, config, logger)
			if err := out.send(result); err != nil {
				logger.Printf("task %s report could not be sent: %v", job.ID, err)
			}
		}(task)
	}
}

func describeDialFailure(response *http.Response, err error) error {
	if response == nil {
		return err
	}
	return fmt.Errorf("%w (server answered %s)", err, response.Status)
}

func (r *reporter) send(result protocol.Report) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode report: %w", err)
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	if err := r.conn.SetWriteDeadline(time.Now().Add(writeReportWait)); err != nil {
		return fmt.Errorf("write deadline: %w", err)
	}
	return r.conn.WriteMessage(websocket.TextMessage, encoded)
}

func run(task protocol.Task, config settings, logger *log.Logger) protocol.Report {
	if len(task.Argv) == 0 {
		logger.Printf("task %s carries no argument list", task.ID)
		return protocol.Report{ID: task.ID, Status: protocol.StatusFailed, Error: "task carries no argument list"}
	}

	timeout := time.Duration(task.Time)*time.Second + config.grace
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	command := exec.CommandContext(ctx, task.Argv[0], task.Argv[1:]...)
	command.Dir = config.workDir
	if config.workDir != "" && !filepath.IsAbs(command.Path) {
		command.Path = filepath.Join(config.workDir, command.Path)
	}

	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	logger.Printf("task %s starting method=%s host=%s time=%ds argv=%q", task.ID, task.Method, task.Host, task.Time, task.Argv)

	started := time.Now()
	err := command.Run()
	elapsed := time.Since(started)

	result := protocol.Report{
		ID:       task.ID,
		Stdout:   protocol.Tail(stdout.String(), protocol.OutputTailBytes),
		Stderr:   protocol.Tail(stderr.String(), protocol.OutputTailBytes),
		ExitCode: 0,
	}

	var exitError *exec.ExitError
	switch {
	case err == nil:
		result.Status = protocol.StatusOK
	case errors.As(err, &exitError):
		result.Status = protocol.StatusFailed
		result.ExitCode = exitError.ExitCode()
	case ctx.Err() != nil:
		result.Status = protocol.StatusFailed
		result.ExitCode = -1
		result.Error = fmt.Sprintf("killed after the %s budget expired", timeout)
	default:
		result.Status = protocol.StatusFailed
		result.ExitCode = -1
		result.Error = err.Error()
	}

	logger.Printf("task %s ended status=%s exit=%d after %s", task.ID, result.Status, result.ExitCode, elapsed.Round(time.Millisecond))

	return result
}
