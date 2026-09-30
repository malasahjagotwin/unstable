package arguments

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const (
	SchemeHTTPS = "https"
	SchemeHTTP  = "http"

	PlaceholderHost = "{host}"
	PlaceholderTime = "{time}"
)

var (
	allowedHostChars = regexp.MustCompile(`^[A-Za-z0-9.:/-]+$`)
	allowedMethod    = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	domainPattern    = regexp.MustCompile(`^(?i)[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*\.[a-z]{2,}$`)
)

type Host struct {
	Scheme   string
	Hostname string
}

func (h Host) URL() string {
	return h.Scheme + "://" + h.Hostname
}

func (h Host) NeedsProbe() bool {
	return h.Scheme == ""
}

type Limits struct {
	MaxSeconds int
	AllowedIPs []string
}

type Request struct {
	Key      string
	Host     Host
	Seconds  int
	Method   string
	ClientIP string
}

func Parse(values url.Values, remoteAddr string, limits Limits) (Request, error) {
	key := strings.TrimSpace(values.Get("key"))
	if key == "" {
		return Request{}, errors.New("缺少 key 参数")
	}

	if err := ValidateIP(remoteAddr, limits.AllowedIPs); err != nil {
		return Request{}, err
	}

	host, err := ValidateHost(values.Get("host"))
	if err != nil {
		return Request{}, err
	}

	seconds, err := ValidateTime(values.Get("time"), limits.MaxSeconds)
	if err != nil {
		return Request{}, err
	}

	method := strings.TrimSpace(values.Get("method"))
	if !allowedMethod.MatchString(method) {
		return Request{}, fmt.Errorf("缺少 method 参数或格式不正确：%q", method)
	}

	client := ""
	if ip := clientIPOf(remoteAddr); ip != nil {
		client = ip.String()
	}

	return Request{
		Key:      key,
		Host:     host,
		Seconds:  seconds,
		Method:   method,
		ClientIP: client,
	}, nil
}

func ValidateHost(raw string) (Host, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return Host{}, errors.New("缺少 host 参数")
	}
	if !allowedHostChars.MatchString(trimmed) {
		return Host{}, errors.New("host 含有不允许的字符")
	}

	scheme := ""
	remainder := trimmed
	if index := strings.Index(trimmed, "://"); index >= 0 {
		scheme = strings.ToLower(trimmed[:index])
		remainder = trimmed[index+3:]

		if scheme != SchemeHTTPS && scheme != SchemeHTTP {
			return Host{}, fmt.Errorf("不支持的协议 %q，只允许 %s:// 和 %s://", scheme, SchemeHTTPS, SchemeHTTP)
		}
	}

	hostname, err := validateHostname(remainder)
	if err != nil {
		return Host{}, err
	}

	return Host{Scheme: scheme, Hostname: hostname}, nil
}

func validateHostname(raw string) (string, error) {
	hostname := strings.TrimSuffix(raw, ".")
	if hostname == "" {
		return "", errors.New("去掉协议后 host 为空")
	}
	if strings.Contains(hostname, ":") {
		return "", errors.New("host 不得包含端口号")
	}
	if strings.ContainsAny(hostname, "/?#@[]") {
		return "", fmt.Errorf("host %q 含有不允许的字符", hostname)
	}
	if isIPv4(hostname) {
		return hostname, nil
	}
	if !domainPattern.MatchString(hostname) {
		return "", fmt.Errorf("host %q 不是有效的域名或 IPv4 地址", hostname)
	}
	return strings.ToLower(hostname), nil
}

func ValidateTime(raw string, limit int) (int, error) {
	if raw == "" {
		return 0, errors.New("缺少 time 参数")
	}
	for _, character := range raw {
		if character < '0' || character > '9' {
			return 0, errors.New("time 只能包含数字")
		}
	}

	seconds, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("time %q 不是有效数字", raw)
	}
	if seconds <= 0 {
		return 0, errors.New("time 必须大于零")
	}
	if limit > 0 && seconds > limit {
		return 0, fmt.Errorf("time %d 超过该 key 的上限 %d 秒", seconds, limit)
	}
	return seconds, nil
}

func NormalizeHostname(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if index := strings.Index(trimmed, "://"); index >= 0 {
		trimmed = trimmed[index+3:]
	}
	if index := strings.IndexAny(trimmed, "/?#"); index >= 0 {
		trimmed = trimmed[:index]
	}
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(trimmed), "."))
}

func HostnameOf(raw string) (string, error) {
	hostname := NormalizeHostname(raw)
	if hostname == "" {
		return "", errors.New("条目为空")
	}
	return hostname, nil
}

func ParseCmdTemplate(template, hostURL string, seconds int) ([]string, error) {
	rendered := strings.ReplaceAll(template, PlaceholderHost, hostURL)
	rendered = strings.ReplaceAll(rendered, PlaceholderTime, strconv.Itoa(seconds))

	argv := strings.Fields(rendered)
	if len(argv) == 0 {
		return nil, fmt.Errorf("命令 %q 渲染后得到空的参数列表", template)
	}
	return argv, nil
}
