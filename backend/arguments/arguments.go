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
		return Request{}, errors.New("key is required")
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
		return Request{}, fmt.Errorf("method %q is missing or malformed", method)
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
		return Host{}, errors.New("host is required")
	}
	if !allowedHostChars.MatchString(trimmed) {
		return Host{}, errors.New("host contains characters that are not allowed")
	}

	scheme := ""
	remainder := trimmed
	if index := strings.Index(trimmed, "://"); index >= 0 {
		scheme = strings.ToLower(trimmed[:index])
		remainder = trimmed[index+3:]

		if scheme != SchemeHTTPS && scheme != SchemeHTTP {
			return Host{}, fmt.Errorf("unsupported scheme %q, only %s:// and %s:// are allowed", scheme, SchemeHTTPS, SchemeHTTP)
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
		return "", errors.New("host is empty after the scheme")
	}
	if strings.Contains(hostname, ":") {
		return "", errors.New("host must not contain a port number")
	}
	if strings.ContainsAny(hostname, "/?#@[]") {
		return "", fmt.Errorf("host %q contains characters that are not allowed", hostname)
	}
	if isIPv4(hostname) {
		return hostname, nil
	}
	if !domainPattern.MatchString(hostname) {
		return "", fmt.Errorf("host %q is not a valid domain name or IPv4 address", hostname)
	}
	return strings.ToLower(hostname), nil
}

func ValidateTime(raw string, limit int) (int, error) {
	if raw == "" {
		return 0, errors.New("time is required")
	}
	for _, character := range raw {
		if character < '0' || character > '9' {
			return 0, errors.New("time must contain digits only")
		}
	}

	seconds, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("time %q is not a valid number", raw)
	}
	if seconds <= 0 {
		return 0, errors.New("time must be greater than zero")
	}
	if limit > 0 && seconds > limit {
		return 0, fmt.Errorf("time %d exceeds the limit of %d seconds for this key", seconds, limit)
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
		return "", errors.New("entry is empty")
	}
	return hostname, nil
}

func ParseCmdTemplate(template, hostURL string, seconds int) ([]string, error) {
	rendered := strings.ReplaceAll(template, PlaceholderHost, hostURL)
	rendered = strings.ReplaceAll(rendered, PlaceholderTime, strconv.Itoa(seconds))

	argv := strings.Fields(rendered)
	if len(argv) == 0 {
		return nil, fmt.Errorf("command %q renders to an empty argument list", template)
	}
	return argv, nil
}
