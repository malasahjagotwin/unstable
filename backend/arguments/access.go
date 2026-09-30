package arguments

import (
	"fmt"
	"net"
	"strings"
)

func ValidateIP(remoteAddr string, whitelist []string) error {
	if len(whitelist) == 0 {
		return nil
	}

	client := clientIPOf(remoteAddr)
	if client == nil {
		return fmt.Errorf("client address %q cannot be parsed as an IP", remoteAddr)
	}

	for _, entry := range whitelist {
		if ipAllowed(entry, client) {
			return nil
		}
	}
	return fmt.Errorf("client %s is not in the ip whitelist of this key", client)
}

func ipAllowed(entry string, client net.IP) bool {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return false
	}
	if strings.Contains(entry, "/") {
		_, network, err := net.ParseCIDR(entry)
		return err == nil && network.Contains(client)
	}
	allowed := net.ParseIP(entry)
	return allowed != nil && allowed.Equal(client)
}

func clientIPOf(remoteAddr string) net.IP {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	return net.ParseIP(strings.Trim(host, "[]"))
}

func isIPv4(hostname string) bool {
	ip := net.ParseIP(hostname)
	return ip != nil && ip.To4() != nil
}
