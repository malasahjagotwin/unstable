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

	"unstablestress/backend/internal/cli"
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

	logger.Printf("机器人启动 endpoint=%s work_dir=%s max_concurrent=%d", config.endpoint, config.workDir, config.concurrency)

	backoff := config.reconnect
	for {
		err := serve(config, logger)
		if err != nil {
			logger.Printf("连接 %s 失败：%v", config.endpoint, err)
		}

		logger.Printf("将在 %s 后重试", backoff)
		time.Sleep(backoff)
		backoff = nextBackoff(backoff)
	}
}

func parseFlags() settings {
	var config settings

	set := cli.New("bot")
	set.StringVar(&config.endpoint, "endpoint", defaultEndpoint, "REST API 的 rcon WebSocket 端点")
	set.StringVar(&config.token, "token", os.Getenv("BOT_TOKEN"), "在 "+protocol.BotTokenHeader+" 标头中发送的共享密钥")
	set.StringVar(&config.workDir, "work-dir", executableDir(), "用于解析相对路径命令、并保存其输出的工作目录")
	set.DurationVar(&config.reconnect, "reconnect", defaultReconnect, "重连前的初始等待时长")
	set.DurationVar(&config.grace, "grace", defaultGrace, "在任务时长之外额外宽限的时间，超时后强制结束进程")
	set.IntVar(&config.concurrency, "max-concurrent", defaultConcurrency, "同时运行的任务数量上限")
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
		return fmt.Errorf("建立连接失败：%w", describeDialFailure(response, err))
	}
	defer conn.Close()

	conn.SetReadLimit(readMessageLimit)
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(2 * handshakeTimeout))
	})

	out := &reporter{conn: conn}
	slots := make(chan struct{}, config.concurrency)

	logger.Printf("已连接到 %s", config.endpoint)

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("读取失败：%w", err)
		}

		var task protocol.Task
		if err := json.Unmarshal(data, &task); err != nil {
			logger.Printf("丢弃一条无法解析的任务：%v", err)
			continue
		}

		slots <- struct{}{}
		go func(job protocol.Task) {
			defer func() { <-slots }()
			result := run(job, config, logger)
			if err := out.send(result); err != nil {
				logger.Printf("任务 %s 的执行结果无法上报：%v", job.ID, err)
			}
		}(task)
	}
}

func describeDialFailure(response *http.Response, err error) error {
	if response == nil {
		return err
	}
	return fmt.Errorf("%w（服务端返回 %s）", err, response.Status)
}

func (r *reporter) send(result protocol.Report) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("编码执行结果失败：%w", err)
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	if err := r.conn.SetWriteDeadline(time.Now().Add(writeReportWait)); err != nil {
		return fmt.Errorf("设置写入超时失败：%w", err)
	}
	return r.conn.WriteMessage(websocket.TextMessage, encoded)
}

func run(task protocol.Task, config settings, logger *log.Logger) protocol.Report {
	if len(task.Argv) == 0 {
		logger.Printf("任务 %s 没有携带任何参数", task.ID)
		return protocol.Report{ID: task.ID, Status: protocol.StatusFailed, Error: "任务没有携带任何参数"}
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

	logger.Printf("任务 %s 开始执行 method=%s host=%s time=%ds argv=%q", task.ID, task.Method, task.Host, task.Time, task.Argv)

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
		result.Error = fmt.Sprintf("超过 %s 的时限，已被强制结束", timeout)
	default:
		result.Status = protocol.StatusFailed
		result.ExitCode = -1
		result.Error = err.Error()
	}

	logger.Printf("任务 %s 执行结束 status=%s exit=%d 耗时 %s", task.ID, result.Status, result.ExitCode, elapsed.Round(time.Millisecond))

	return result
}
