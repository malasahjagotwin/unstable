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
		{name: "裸域名需要探测协议", raw: "laoshi.com", hostname: "laoshi.com", scheme: ""},
		{name: "带 https 前缀保持 https", raw: "https://laohi.com", hostname: "laohi.com", scheme: "https"},
		{name: "带 http 前缀保持 http", raw: "http://laohi.com", hostname: "laohi.com", scheme: "http"},
		{name: "协议转小写", raw: "HTTPS://LaoHi.com", hostname: "laohi.com", scheme: "https"},
		{name: "去掉结尾的点", raw: "laoshi.com.", hostname: "laoshi.com", scheme: ""},
		{name: "允许多级子域名", raw: "a.b.example.co.uk", hostname: "a.b.example.co.uk", scheme: ""},
		{name: "允许 IPv4", raw: "http://93.184.216.34", hostname: "93.184.216.34", scheme: "http"},

		{name: "拒绝 http 带端口", raw: "http://laohi.com:8080", rejected: true},
		{name: "拒绝 https 带端口", raw: "https://laoshgo.com:707", rejected: true},
		{name: "拒绝裸端口", raw: "laohi.com:80", rejected: true},
		{name: "拒绝带路径", raw: "laohi.com/admin", rejected: true},
		{name: "拒绝带 userinfo", raw: "user@laohi.com", rejected: true},
		{name: "拒绝空格", raw: "laohi .com", rejected: true},
		{name: "拒绝分号", raw: "laohi.com;whoami", rejected: true},
		{name: "拒绝反引号", raw: "`id`", rejected: true},
		{name: "拒绝竖线", raw: "laohi.com|id", rejected: true},
		{name: "拒绝美元符号", raw: "$HOME", rejected: true},
		{name: "拒绝换行符", raw: "laohi.com\nid", rejected: true},
		{name: "拒绝不支持的协议", raw: "ftp://laohi.com", rejected: true},
		{name: "拒绝单段主机名", raw: "localhost", rejected: true},
		{name: "拒绝数字顶级域", raw: "laohi.123", rejected: true},
		{name: "拒绝空 host", raw: "", rejected: true},
		{name: "拒绝只有协议没有主机名", raw: "https://", rejected: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			host, err := ValidateHost(testCase.raw)

			if testCase.rejected {
				if err == nil {
					t.Fatalf("ValidateHost(%q) 被接受，应当被拒绝", testCase.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateHost(%q) 失败：%v", testCase.raw, err)
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
		{name: "低于上限", raw: "80", limit: 120, expected: 80},
		{name: "正好等于上限", raw: "120", limit: 120, expected: 120},
		{name: "超出上限一秒", raw: "121", limit: 120, rejected: true},
		{name: "拒绝带点的输入", raw: "1.1.1", limit: 120, rejected: true},
		{name: "拒绝含字母", raw: "12a", limit: 120, rejected: true},
		{name: "拒绝带单位", raw: "10s", limit: 120, rejected: true},
		{name: "拒绝正负号", raw: "-10", limit: 120, rejected: true},
		{name: "拒绝空值", raw: "", limit: 120, rejected: true},
		{name: "拒绝零", raw: "0", limit: 120, rejected: true},
		{name: "无上限时接受大数值", raw: "9999", limit: 0, expected: 9999},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			seconds, err := ValidateTime(testCase.raw, testCase.limit)

			if testCase.rejected {
				if err == nil {
					t.Fatalf("ValidateTime(%q, %d) 被接受，应当被拒绝", testCase.raw, testCase.limit)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateTime(%q, %d) 失败：%v", testCase.raw, testCase.limit, err)
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
		{name: "精确匹配", remoteAddr: "10.0.0.15:5555", whitelist: whitelist},
		{name: "IPv6 回环地址", remoteAddr: "[::1]:5555", whitelist: []string{"::1"}},
		{name: "落在 CIDR 段内", remoteAddr: "192.168.1.77:5555", whitelist: whitelist},
		{name: "不在白名单内", remoteAddr: "8.8.8.8:5555", whitelist: whitelist, rejected: true},
		{name: "落在 CIDR 段外", remoteAddr: "192.168.2.5:5555", whitelist: whitelist, rejected: true},
		{name: "白名单为空时放行所有来源", remoteAddr: "8.8.8.8:5555", whitelist: nil},
		{name: "无法解析的地址", remoteAddr: "not-an-ip", whitelist: whitelist, rejected: true},
		{name: "无白名单时的无法解析地址", remoteAddr: "not-an-ip", whitelist: nil},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := ValidateIP(testCase.remoteAddr, testCase.whitelist)

			switch {
			case testCase.rejected && err == nil:
				t.Fatalf("ValidateIP(%q, %v) 被接受，应当被拒绝", testCase.remoteAddr, testCase.whitelist)
			case !testCase.rejected && err != nil:
				t.Fatalf("ValidateIP(%q, %v) 失败：%v", testCase.remoteAddr, testCase.whitelist, err)
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
			name:     "不含占位符的速率值原样保留",
			template: "./liu {host} {time} 10300",
			host:     "http://laohi.com",
			seconds:  10,
			expected: []string{"./liu", "http://laohi.com", "10", "10300"},
		},
		{name: "拒绝空模板", template: "   ", rejected: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			argv, err := ParseCmdTemplate(testCase.template, testCase.host, testCase.seconds)

			if testCase.rejected {
				if err == nil {
					t.Fatalf("ParseCmdTemplate(%q) 被接受，应当被拒绝", testCase.template)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseCmdTemplate(%q) 失败：%v", testCase.template, err)
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
			t.Fatalf("Parse 失败：%v", err)
		}
		if request.Seconds != 80 || request.Host.Hostname != "laoshi.com" || request.Method != "tls" {
			t.Fatalf("Parse = %+v，应为 seconds=80 host=laoshi.com method=tls", request)
		}
		if request.ClientIP != "127.0.0.1" {
			t.Fatalf("Parse 得到的客户端 IP = %q，应为 127.0.0.1", request.ClientIP)
		}
		if !request.Host.NeedsProbe() {
			t.Fatal("裸域名仍然需要协议探测")
		}
	})

	t.Run("keeps an explicit scheme", func(t *testing.T) {
		values := url.Values{"key": {"k"}, "host": {"http://laohi.com"}, "time": {"10"}, "method": {"tls"}}

		request, err := Parse(values, "127.0.0.1:1", limits)
		if err != nil {
			t.Fatalf("Parse 失败：%v", err)
		}
		if request.Host.NeedsProbe() {
			t.Fatal("显式指定协议时不应触发探测")
		}
		if request.Host.URL() != "http://laohi.com" {
			t.Fatalf("URL = %q，应为 http://laohi.com", request.Host.URL())
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
				t.Fatalf("Parse(%v) 被接受，应当被拒绝", values)
			}
		})
	}

	t.Run("rejects a foreign client ip", func(t *testing.T) {
		values := url.Values{"key": {"k"}, "host": {"laohi.com"}, "time": {"10"}, "method": {"tls"}}

		if _, err := Parse(values, "8.8.8.8:1", limits); err == nil {
			t.Fatal("来自非白名单地址的 Parse 被接受，应当被拒绝")
		}
	})
}

func TestNewProberRejectsBadTimeouts(t *testing.T) {
	for _, raw := range []string{"", "soon", "0s", "-1s"} {
		if _, err := NewProber(raw); err == nil {
			t.Fatalf("NewProber(%q) 被接受，应当被拒绝", raw)
		}
	}
	if _, err := NewProber("250ms"); err != nil {
		t.Fatalf("NewProber(\"250ms\") 失败：%v", err)
	}
}
