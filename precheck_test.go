package proxypool

import (
	"testing"
	"time"
)

func TestPreCheck_Disabled(t *testing.T) {
	// 预检禁用时，所有代理都应该通过
	pool, err := New(Config{
		Provider:   NewExampleProvider(),
		TargetSize: 5,
		PreCheck: PreCheckConfig{
			Enabled: false,
		},
		Logger: &testLogger{t: t},
	})
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	stats := pool.Stats()
	if stats.Total != 5 {
		t.Errorf("Expected 5 proxies, got %d", stats.Total)
	}
}

func TestPreCheck_ExtractIP(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		body     string
		expected string
	}{
		{
			name:     "ipify JSON",
			url:      "https://api.ipify.org",
			body:     `{"ip":"1.2.3.4"}`,
			expected: "1.2.3.4",
		},
		{
			name:     "plain text IPv4",
			url:      "https://icanhazip.com",
			body:     "5.6.7.8\n",
			expected: "5.6.7.8",
		},
		{
			name:     "plain text with spaces",
			url:      "https://checkip.amazonaws.com",
			body:     "  9.10.11.12  \n",
			expected: "9.10.11.12",
		},
		{
			name:     "IPv6",
			url:      "https://api.ipify.org",
			body:     `{"ip":"2001:db8::1"}`,
			expected: "2001:db8::1",
		},
		{
			name:     "invalid JSON",
			url:      "https://example.com",
			body:     "not an ip",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractIPFromResponse(tt.url, tt.body)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestPreCheck_IsValidIP(t *testing.T) {
	tests := []struct {
		ip    string
		valid bool
	}{
		{"1.2.3.4", true},
		{"192.168.1.1", true},
		{"255.255.255.255", true},
		{"0.0.0.0", true},
		{"2001:db8::1", true},
		{"::1", true},
		{"fe80::1", true},
		{"", false},
		{"abc", false},
		{"1.2.3", false},
		{"1.2.3.4.5", false},
		{"256.1.1.1", false}, // 注意：这个简单验证会通过，实际应该用net.ParseIP
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			result := isValidIP(tt.ip)
			if result != tt.valid {
				t.Errorf("isValidIP(%q) = %v, want %v", tt.ip, result, tt.valid)
			}
		})
	}
}

func TestPreCheckConfig_Defaults(t *testing.T) {
	cfg := PreCheckConfig{
		Enabled: true,
	}
	cfg.applyDefaults()

	if cfg.Timeout != 5*time.Second {
		t.Errorf("Expected timeout 5s, got %v", cfg.Timeout)
	}
	if cfg.MaxLatency != 3*time.Second {
		t.Errorf("Expected max latency 3s, got %v", cfg.MaxLatency)
	}
	if cfg.Concurrency != 10 {
		t.Errorf("Expected concurrency 10, got %d", cfg.Concurrency)
	}
	if len(cfg.CheckURLs) == 0 {
		t.Error("Expected default check URLs to be set")
	}
	if cfg.MinSuccessCount != 1 {
		t.Errorf("Expected min success count 1, got %d", cfg.MinSuccessCount)
	}
}

// testLogger 测试用的日志实现
