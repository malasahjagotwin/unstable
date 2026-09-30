package arguments

import (
	"crypto/tls"
	"fmt"
	"net"
	"time"
)

type Prober struct {
	timeout time.Duration
}

func NewProber(raw string) (*Prober, error) {
	timeout, err := time.ParseDuration(raw)
	if err != nil {
		return nil, fmt.Errorf("探测超时 %q 不是有效的时间长度：%w", raw, err)
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("探测超时 %q 必须大于零", raw)
	}
	return &Prober{timeout: timeout}, nil
}

func (p *Prober) Resolve(host Host) Host {
	if !host.NeedsProbe() {
		return host
	}

	scheme := SchemeHTTP
	if p.servesTLS(host.Hostname) {
		scheme = SchemeHTTPS
	}
	return Host{Scheme: scheme, Hostname: host.Hostname}
}

func (p *Prober) servesTLS(hostname string) bool {
	connection, err := net.DialTimeout("tcp", net.JoinHostPort(hostname, "443"), p.timeout)
	if err != nil {
		return false
	}
	defer connection.Close()

	if err := connection.SetDeadline(time.Now().Add(p.timeout)); err != nil {
		return false
	}

	client := tls.Client(connection, &tls.Config{
		ServerName: hostname,
		MinVersion: tls.VersionTLS12,

		InsecureSkipVerify: true,
	})
	defer client.Close()

	return client.Handshake() == nil
}
