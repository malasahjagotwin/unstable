package main

import (
	"fmt"
	"sync"
	"time"
)

type slotManager struct {
	mutex   sync.Mutex
	running map[string]map[string]time.Time
}

func newSlotManager() *slotManager {
	return &slotManager{running: make(map[string]map[string]time.Time)}
}

func (m *slotManager) acquire(key, taskID string, ttl time.Duration, limit int) (int, error) {
	if limit <= 0 {
		limit = 1
	}

	now := time.Now()

	m.mutex.Lock()
	defer m.mutex.Unlock()

	deadlines := m.liveDeadlines(key, now)
	if len(deadlines) >= limit {
		return 0, fmt.Errorf("all %d slot(s) for this key are occupied, retry after a running task expires", limit)
	}

	deadlines[taskID] = now.Add(ttl)
	return limit - len(deadlines), nil
}

func (m *slotManager) usage(key string) int {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	return len(m.liveDeadlines(key, time.Now()))
}

func (m *slotManager) release(key, taskID string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if deadlines, present := m.running[key]; present {
		delete(deadlines, taskID)
		if len(deadlines) == 0 {
			delete(m.running, key)
		}
	}
}

func (m *slotManager) reap() {
	now := time.Now()

	m.mutex.Lock()
	defer m.mutex.Unlock()

	for key, deadlines := range m.running {
		for taskID, deadline := range deadlines {
			if now.After(deadline) {
				delete(deadlines, taskID)
			}
		}
		if len(deadlines) == 0 {
			delete(m.running, key)
		}
	}
}

func (m *slotManager) liveDeadlines(key string, now time.Time) map[string]time.Time {
	deadlines := m.running[key]
	if deadlines == nil {
		deadlines = make(map[string]time.Time)
		m.running[key] = deadlines
	}
	for taskID, deadline := range deadlines {
		if now.After(deadline) {
			delete(deadlines, taskID)
		}
	}
	return deadlines
}

func (m *slotManager) reapLoop(stop <-chan struct{}, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			m.reap()
		}
	}
}
