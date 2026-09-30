package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"unstablestress/backend/internal/protocol"
)

const (
	botSendQueueSize = 16
	writeWait        = 10 * time.Second
	pongWait         = 60 * time.Second
	pingInterval     = 25 * time.Second
)

type botConn struct {
	id     string
	conn   *websocket.Conn
	send   chan []byte
	hub    *hub
	joined time.Time
}

type hub struct {
	token    string
	logger   *log.Logger
	upgrader websocket.Upgrader

	mutex  sync.RWMutex
	bots   map[*botConn]struct{}
	serial uint64
}

func newHub(token string, logger *log.Logger) *hub {
	return &hub{
		token:  token,
		logger: logger,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin:     func(*http.Request) bool { return true },
		},
		bots: make(map[*botConn]struct{}),
	}
}

func (h *hub) authorized(r *http.Request) bool {
	return h.token == "" || r.Header.Get(protocol.BotTokenHeader) == h.token
}

func (h *hub) handle(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	connection, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Printf("rcon upgrade from %s failed: %v", r.RemoteAddr, err)
		return
	}

	bot := &botConn{
		id:     h.nextID(),
		conn:   connection,
		send:   make(chan []byte, botSendQueueSize),
		hub:    h,
		joined: time.Now(),
	}

	if err := connection.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		h.logger.Printf("bot %s read deadline setup failed: %v", bot.id, err)
	}
	connection.SetPongHandler(func(string) error {
		return connection.SetReadDeadline(time.Now().Add(pongWait))
	})

	h.register(bot)
	h.logger.Printf("bot %s connected from %s (online=%d)", bot.id, r.RemoteAddr, h.count())

	go bot.writeLoop()
	bot.readLoop()
}

func (h *hub) broadcast(payload protocol.Task) int {
	encoded, err := json.Marshal(payload)
	if err != nil {
		h.logger.Printf("task %s cannot be encoded: %v", payload.ID, err)
		return 0
	}

	h.mutex.RLock()
	defer h.mutex.RUnlock()

	delivered := 0
	for bot := range h.bots {
		select {
		case bot.send <- encoded:
			delivered++
		default:
			h.logger.Printf("bot %s queue is full, task %s skipped for it", bot.id, payload.ID)
		}
	}
	return delivered
}

func (h *hub) count() int {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	return len(h.bots)
}

func (h *hub) register(bot *botConn) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.bots[bot] = struct{}{}
}

func (h *hub) remove(bot *botConn) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if _, present := h.bots[bot]; !present {
		return
	}
	delete(h.bots, bot)
	close(bot.send)
}

func (h *hub) nextID() string {
	h.mutex.Lock()
	h.serial++
	serial := h.serial
	h.mutex.Unlock()

	buffer := make([]byte, 3)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("bot-%04d", serial)
	}
	return fmt.Sprintf("bot-%04d-%s", serial, hex.EncodeToString(buffer))
}

func (b *botConn) readLoop() {
	defer func() {
		b.hub.remove(b)
		b.conn.Close()
		b.hub.logger.Printf("bot %s disconnected after %s (online=%d)",
			b.id, time.Since(b.joined).Round(time.Second), b.hub.count())
	}()

	for {
		_, data, err := b.conn.ReadMessage()
		if err != nil {
			return
		}

		var result protocol.Report
		if err := json.Unmarshal(data, &result); err != nil {
			b.hub.logger.Printf("bot %s sent an unreadable report: %v", b.id, err)
			continue
		}
		result.BotID = b.id
		b.hub.logReport(result)
	}
}

func (b *botConn) writeLoop() {
	ticker := time.NewTicker(pingInterval)
	defer func() {
		ticker.Stop()
		b.conn.Close()
	}()

	for {
		select {
		case message, open := <-b.send:
			if !open {
				b.conn.WriteControl(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
					time.Now().Add(writeWait))
				return
			}
			if err := b.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return
			}
			if err := b.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			if err := b.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait)); err != nil {
				return
			}
		}
	}
}

func (h *hub) logReport(result protocol.Report) {
	if result.Status == protocol.StatusOK {
		h.logger.Printf("task %s finished on %s exit=%d stdout=%q",
			result.ID, result.BotID, result.ExitCode, protocol.Tail(result.Stdout, protocol.OutputTailBytes))
		return
	}
	h.logger.Printf("task %s failed on %s exit=%d error=%q stderr=%q",
		result.ID, result.BotID, result.ExitCode, result.Error, protocol.Tail(result.Stderr, protocol.OutputTailBytes))
}
