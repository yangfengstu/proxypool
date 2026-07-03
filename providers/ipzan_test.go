package providers

import (
	"context"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/yangfengstu/proxypool"
)

func TestIPZanProvider_Fetch(t *testing.T) {
	no := os.Getenv("IPZAN_NO")
	secret := os.Getenv("IPZAN_SECRET")
	if no == "" || secret == "" {
		t.Skip("set IPZAN_NO and IPZAN_SECRET to run live provider fetch test")
	}

	// 使用示例配置（实际测试时需要真实的订单号和密钥）
	provider, err := NewIPZanProvider(IPZanConfig{
		No:       no,
		Secret:   secret,
		Minute:   3,
		Protocol: 3, // SOCKS5
	})
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// 拉取代理
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	proxies, err := provider.Fetch(ctx, 2)
	if err != nil {
		t.Fatalf("Failed to fetch proxies: %v", err)
	}

	if len(proxies) == 0 {
		t.Fatal("No proxies returned")
	}

	// 验证代理字段
	for i, proxy := range proxies {
		t.Logf("Proxy %d:", i+1)
		t.Logf("  Type: %s", proxy.Type)
		t.Logf("  Host: %s", proxy.Host)
		t.Logf("  Port: %d", proxy.Port)
		t.Logf("  Username: %s", proxy.Username)
		t.Logf("  Password: %s", proxy.Password)
		t.Logf("  ExpiredAt: %s", proxy.ExpiredAt.Format("2006-01-02 15:04:05"))
		t.Logf("  ISP: %s", proxy.ISP)
		t.Logf("  URL: %s", proxy.URL())

		// 基本验证
		if proxy.Host == "" {
			t.Errorf("Proxy %d: empty host", i+1)
		}
		if proxy.Port == 0 {
			t.Errorf("Proxy %d: invalid port", i+1)
		}
		if proxy.Type != proxypool.ProxyTypeSOCKS5 {
			t.Errorf("Proxy %d: expected SOCKS5, got %s", i+1, proxy.Type)
		}
		if proxy.ExpiredAt.Before(time.Now()) {
			t.Errorf("Proxy %d: already expired", i+1)
		}
	}
}

func TestIPZanProvider_Config(t *testing.T) {
	tests := []struct {
		name    string
		config  IPZanConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: IPZanConfig{
				No:     "test-no",
				Secret: "test-secret",
			},
			wantErr: false,
		},
		{
			name: "missing no",
			config: IPZanConfig{
				Secret: "test-secret",
			},
			wantErr: true,
			errMsg:  "order number",
		},
		{
			name: "missing secret",
			config: IPZanConfig{
				No: "test-no",
			},
			wantErr: true,
			errMsg:  "secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewIPZanProvider(tt.config)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestIPZanProvider_Protocol(t *testing.T) {
	tests := []struct {
		name     string
		protocol int
		expected proxypool.ProxyType
	}{
		{"HTTP", 1, proxypool.ProxyTypeHTTP},
		{"SOCKS5", 3, proxypool.ProxyTypeSOCKS5},
		{"default", 0, proxypool.ProxyTypeSOCKS5}, // 默认SOCKS5
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, _ := NewIPZanProvider(IPZanConfig{
				No:       "test-no",
				Secret:   "test-secret",
				Protocol: tt.protocol,
			})
			parsedURL, _ := url.Parse(provider.baseURL)
			protocol := parsedURL.Query().Get("protocol")

			// 验证协议映射是否正确
			var proxyType proxypool.ProxyType
			switch protocol {
			case "1":
				proxyType = proxypool.ProxyTypeHTTP
			case "3":
				proxyType = proxypool.ProxyTypeSOCKS5
			default:
				proxyType = proxypool.ProxyTypeSOCKS5
			}

			if proxyType != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, proxyType)
			}
		})
	}
}
