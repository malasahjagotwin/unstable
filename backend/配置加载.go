package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"unstablestress/backend/internal/arguments"
)

type flexInt int

func (f *flexInt) UnmarshalJSON(data []byte) error {
	text := strings.Trim(strings.TrimSpace(string(data)), `"`)
	if text == "" || text == "null" {
		*f = 0
		return nil
	}
	value, err := strconv.Atoi(text)
	if err != nil {
		return fmt.Errorf("期望一个数字，实际得到 %s", string(data))
	}
	*f = flexInt(value)
	return nil
}

type User struct {
	Key         string   `json:"key"`
	Time        flexInt  `json:"time"`
	Slot        flexInt  `json:"slot"`
	IPWhitelist []string `json:"ipwhitelist"`
}

type Method struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      bool   `json:"status"`
	Cmd         string `json:"cmd"`
}

type Store struct {
	Users     []User
	Methods   []Method
	Blacklist []string

	blockedHosts map[string]struct{}
}

func (s *Store) User(key string) (User, bool) {
	for _, candidate := range s.Users {
		if candidate.Key == key {
			return candidate, true
		}
	}
	return User{}, false
}

func (s *Store) Method(name string) (Method, bool) {
	for _, candidate := range s.Methods {
		if candidate.Name == name {
			return candidate, true
		}
	}
	return Method{}, false
}

func (s *Store) HostBlocked(hostname string) bool {
	_, blocked := s.blockedHosts[arguments.NormalizeHostname(hostname)]
	return blocked
}

func LoadStore(jsonDir string) (*Store, error) {
	resolved, err := resolveJSONDir(jsonDir)
	if err != nil {
		return nil, err
	}

	store := &Store{}
	if err := readJSONFile(filepath.Join(resolved, "users.json"), &store.Users); err != nil {
		return nil, err
	}
	if err := readJSONFile(filepath.Join(resolved, "methods.json"), &store.Methods); err != nil {
		return nil, err
	}
	if err := readJSONFile(filepath.Join(resolved, "blacklist.json"), &store.Blacklist); err != nil {
		return nil, err
	}

	if len(store.Users) == 0 {
		return nil, fmt.Errorf("users.json 中没有任何用户")
	}
	if len(store.Methods) == 0 {
		return nil, fmt.Errorf("methods.json 中没有任何方法")
	}

	store.blockedHosts = make(map[string]struct{}, len(store.Blacklist))
	for _, entry := range store.Blacklist {
		hostname, err := arguments.HostnameOf(entry)
		if err != nil {
			return nil, fmt.Errorf("blacklist.json 中的条目 %q：%w", entry, err)
		}
		store.blockedHosts[hostname] = struct{}{}
	}

	return store, nil
}

func resolveJSONDir(jsonDir string) (string, error) {
	if info, err := os.Stat(jsonDir); err == nil && info.IsDir() {
		return jsonDir, nil
	}

	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("找不到 json 目录 %q，且无法确定可执行文件路径：%w", jsonDir, err)
	}
	fallback := filepath.Join(filepath.Dir(executable), jsonDir)
	if info, err := os.Stat(fallback); err == nil && info.IsDir() {
		return fallback, nil
	}

	return "", fmt.Errorf("找不到 json 目录 %q（同时检查过 %q）", jsonDir, fallback)
}

func readJSONFile(path string, target any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取 %s 失败：%w", path, err)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("解析 %s 失败：%w", path, err)
	}
	return nil
}
