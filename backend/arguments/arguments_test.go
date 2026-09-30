package arguments

import (
	"net/url"
	"reflect"
	"testing"
)

func TestValidateHost(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		hostname string
		scheme   string
		rejected bool
	}{
		{name: "bare domain needs a probe", raw: "laoshi.com", hostname: "laoshi.com", scheme: ""},
		{name: "https stays https", raw: "https://laohi.com", hostname: "laohi.com", scheme: "https"},
		{name: "http stays http", raw: "http://laohi.com", hostname: "laohi.com", scheme: "http"},
		{name: "scheme is lowercased", raw: "HTTPS://LaoHi.com", hostname: "laohi.com", scheme: "https"},
		{name: "trailing dot is dropped", raw: "laoshi.com.", hostname: "laoshi.com", scheme: ""},
		{name: "subdomains are allowed", raw: "a.b.example.co.uk", hostname: "a.b.example.co.uk", scheme: ""},
		{name: "ipv4 is allowed", raw: "http://93.184.216.34", hostname: "93.184.216.34", scheme: "http"},

		{name: "rejects an http port", raw: "http://laohi.com:8080", rejected: true},
		{name: "rejects an https port", raw: "https://laoshgo.com:707", rejected: true},
		{name: "rejects a bare port", raw: "laohi.com:80", rejected: true},
		{name: "rejects a path", raw: "laohi.com/admin", rejected: true},
		{name: "rejects userinfo", raw: "user@laohi.com", rejected: true},
		{name: "rejects a space", raw: "laohi .com", rejected: true},
		{name: "rejects a semicolon", raw: "laohi.com;whoami", rejected: true},
		{name: "rejects a backtick", raw: "`id`", rejected: true},
		{name: "rejects a pipe", raw: "laohi.com|id", rejected: true},
		{name: "rejects a dollar", raw: "$HOME", rejected: true},
		{name: "rejects a newline", raw: "laohi.com\nid", rejected: true},
		{name: "rejects an unsupported scheme", raw: "ftp://laohi.com", rejected: true},
		{name: "rejects a single label", raw: "localhost", rejected: true},
		{name: "rejects a numeric tld", raw: "laohi.123", rejected: true},
		{name: "rejects an empty host", raw: "", rejected: true},
		{name: "rejects a bare scheme", raw: "https://", rejected: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			host, err := ValidateHost(testCase.raw)

			if testCase.rejected {
				if err == nil {
					t.Fatalf("ValidateHost(%q) was accepted, want rejection", testCase.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateHost(%q) failed: %v", testCase.raw, err)
			}
			if host.Hostname != testCase.hostname || host.Scheme != testCase.scheme {
				t.Fatalf("ValidateHost(%q) = %+v, want hostname=%q scheme=%q",
					testCase.raw, host, testCase.hostname, testCase.scheme)
			}
		})
	}
}

func TestValidateTime(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		limit    int
		expected int
		rejected bool
	}{
		{name: "under the limit", raw: "80", limit: 120, expected: 80},
		{name: "exactly the limit", raw: "120", limit: 120, expected: 120},
		{name: "one over the limit", raw: "121", limit: 120, rejected: true},
		{name: "dotted input is rejected", raw: "1.1.1", limit: 120, rejected: true},
		{name: "letters are rejected", raw: "12a", limit: 120, rejected: true},
		{name: "units are rejected", raw: "10s", limit: 120, rejected: true},
		{name: "signs are rejected", raw: "-10", limit: 120, rejected: true},
		{name: "empty is rejected", raw: "", limit: 120, rejected: true},
		{name: "zero is rejected", raw: "0", limit: 120, rejected: true},
		{name: "no limit accepts a large value", raw: "9999", limit: 0, expected: 9999},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			seconds, err := ValidateTime(testCase.raw, testCase.limit)

			if testCase.rejected {
				if err == nil {
					t.Fatalf("ValidateTime(%q, %d) was accepted, want rejection", testCase.raw, testCase.limit)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateTime(%q, %d) failed: %v", testCase.raw, testCase.limit, err)
			}
			if seconds != testCase.expected {
				t.Fatalf("ValidateTime(%q, %d) = %d, want %d", testCase.raw, testCase.limit, seconds, testCase.expected)
			}
		})
	}
}

func TestValidateIP(t *testing.T) {
	whitelist := []string{"127.0.0.1", "10.0.0.15", "192.168.1.0/24"}

	cases := []struct {
		name       string
		remoteAddr string
		whitelist  []string
		rejected   bool
	}{
		{name: "exact match", remoteAddr: "10.0.0.15:5555", whitelist: whitelist},
		{name: "ipv6 loopback", remoteAddr: "[::1]:5555", whitelist: []string{"::1"}},
		{name: "inside a cidr", remoteAddr: "192.168.1.77:5555", whitelist: whitelist},
		{name: "outside the whitelist", remoteAddr: "8.8.8.8:5555", whitelist: whitelist, rejected: true},
		{name: "outside the cidr", remoteAddr: "192.168.2.5:5555", whitelist: whitelist, rejected: true},
		{name: "empty whitelist allows anyone", remoteAddr: "8.8.8.8:5555", whitelist: nil},
		{name: "unparsable address", remoteAddr: "not-an-ip", whitelist: whitelist, rejected: true},
		{name: "unparsable address without a whitelist", remoteAddr: "not-an-ip", whitelist: nil},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := ValidateIP(testCase.remoteAddr, testCase.whitelist)

			switch {
			case testCase.rejected && err == nil:
				t.Fatalf("ValidateIP(%q, %v) was accepted, want rejection", testCase.remoteAddr, testCase.whitelist)
			case !testCase.rejected && err != nil:
				t.Fatalf("ValidateIP(%q, %v) failed: %v", testCase.remoteAddr, testCase.whitelist, err)
			}
		})
	}
}

func TestParseCmdTemplate(t *testing.T) {
	cases := []struct {
		name     string
		template string
		host     string
		seconds  int
		expected []string
		rejected bool
	}{
		{
			name:     "renders the documented template",
			template: "./liu {host} {time} 1000",
			host:     "https://laohi.com",
			seconds:  80,
			expected: []string{"./liu", "https://laohi.com", "80", "1000"},
		},
		{
			name:     "a rate without placeholders is kept",
			template: "./liu {host} {time} 10300",
			host:     "http://laohi.com",
			seconds:  10,
			expected: []string{"./liu", "http://laohi.com", "10", "10300"},
		},
		{name: "an empty template is rejected", template: "   ", rejected: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			argv, err := ParseCmdTemplate(testCase.template, testCase.host, testCase.seconds)

			if testCase.rejected {
				if err == nil {
					t.Fatalf("ParseCmdTemplate(%q) was accepted, want rejection", testCase.template)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseCmdTemplate(%q) failed: %v", testCase.template, err)
			}
			if !reflect.DeepEqual(argv, testCase.expected) {
				t.Fatalf("ParseCmdTemplate(%q) = %v, want %v", testCase.template, argv, testCase.expected)
			}
		})
	}
}

func TestNormalizeHostname(t *testing.T) {
	cases := map[string]string{
		"http://nginx.com":   "nginx.com",
		"https://NGINX.com":  "nginx.com",
		"nginx.com":          "nginx.com",
		"https://nginx.com/": "nginx.com",
		"nginx.com.":         "nginx.com",
	}

	for raw, expected := range cases {
		if got := NormalizeHostname(raw); got != expected {
			t.Fatalf("NormalizeHostname(%q) = %q, want %q", raw, got, expected)
		}
	}
}

func TestParse(t *testing.T) {
	limits := Limits{MaxSeconds: 120, AllowedIPs: []string{"127.0.0.1"}}

	t.Run("accepts a well formed request", func(t *testing.T) {
		values := url.Values{
			"key":    {"usr_k8f9a2b4c1d6e3f5"},
			"host":   {"laoshi.com"},
			"time":   {"80"},
			"method": {"tls"},
		}

		request, err := Parse(values, "127.0.0.1:41234", limits)
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		if request.Seconds != 80 || request.Host.Hostname != "laoshi.com" || request.Method != "tls" {
			t.Fatalf("Parse = %+v, want seconds=80 host=laoshi.com method=tls", request)
		}
		if request.ClientIP != "127.0.0.1" {
			t.Fatalf("Parse client ip = %q, want 127.0.0.1", request.ClientIP)
		}
		if !request.Host.NeedsProbe() {
			t.Fatal("a bare domain must still need a protocol probe")
		}
	})

	t.Run("keeps an explicit scheme", func(t *testing.T) {
		values := url.Values{"key": {"k"}, "host": {"http://laohi.com"}, "time": {"10"}, "method": {"tls"}}

		request, err := Parse(values, "127.0.0.1:1", limits)
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		if request.Host.NeedsProbe() {
			t.Fatal("an explicit scheme must not trigger a probe")
		}
		if request.Host.URL() != "http://laohi.com" {
			t.Fatalf("URL = %q, want http://laohi.com", request.Host.URL())
		}
	})

	rejections := map[string]url.Values{
		"missing key":         {"host": {"laohi.com"}, "time": {"10"}, "method": {"tls"}},
		"missing host":        {"key": {"k"}, "time": {"10"}, "method": {"tls"}},
		"host with a port":    {"key": {"k"}, "host": {"http://laohi.com:8080"}, "time": {"10"}, "method": {"tls"}},
		"dotted time":         {"key": {"k"}, "host": {"laohi.com"}, "time": {"1.1.1"}, "method": {"tls"}},
		"time over the limit": {"key": {"k"}, "host": {"laohi.com"}, "time": {"121"}, "method": {"tls"}},
		"missing method":      {"key": {"k"}, "host": {"laohi.com"}, "time": {"10"}},
		"malformed method":    {"key": {"k"}, "host": {"laohi.com"}, "time": {"10"}, "method": {"tls; rm -rf /"}},
	}

	for name, values := range rejections {
		t.Run("rejects "+name, func(t *testing.T) {
			if _, err := Parse(values, "127.0.0.1:1", limits); err == nil {
				t.Fatalf("Parse(%v) was accepted, want rejection", values)
			}
		})
	}

	t.Run("rejects a foreign client ip", func(t *testing.T) {
		values := url.Values{"key": {"k"}, "host": {"laohi.com"}, "time": {"10"}, "method": {"tls"}}

		if _, err := Parse(values, "8.8.8.8:1", limits); err == nil {
			t.Fatal("Parse from a non whitelisted address was accepted, want rejection")
		}
	})
}

func TestNewProberRejectsBadTimeouts(t *testing.T) {
	for _, raw := range []string{"", "soon", "0s", "-1s"} {
		if _, err := NewProber(raw); err == nil {
			t.Fatalf("NewProber(%q) was accepted, want rejection", raw)
		}
	}
	if _, err := NewProber("250ms"); err != nil {
		t.Fatalf("NewProber(\"250ms\") failed: %v", err)
	}
}
