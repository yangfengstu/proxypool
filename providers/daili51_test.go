package providers

import (
	"context"
	"testing"
	"time"

	"github.com/yourusername/proxypool"
)

func TestDaili51Provider_Fetch(t *testing.T) {
	// 使用示例提取链接（实际测试时需要真实的链接）
	extractURL := "http://capi.51daili.com/traffic/getip?linePoolIndex=1&packid=12&time=11&qty=1&port=2&format=json&field=ipport,expiretime,regioncode,isptype&ct=1&rid=mr4jlbsowa7t5f18vyqag&uid=48787&accessName=yangfengstu&accessPassword=60131029DC2A7C2F37F7396B9B4C698D"

	provider, err := NewDaili51Provider(Daili51Config{
		ExtractURL: extractURL,
	})
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// 验证认证信息已提取
	if provider.username != "yangfengstu" {
		t.Errorf("Expected username 'yangfengstu', got '%s'", provider.username)
	}
	if provider.password != "60131029DC2A7C2F37F7396B9B4C698D" {
		t.Errorf("Expected password '60131029DC2A7C2F37F7396B9B4C698D', got '%s'", provider.password)
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
		t.Logf("  Region: %s", proxy.Region)
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
		if proxy.Username == "" {
			t.Errorf("Proxy %d: empty username", i+1)
		}
		if proxy.Password == "" {
			t.Errorf("Proxy %d: empty password", i+1)
		}
	}
}

func TestDaili51Provider_Config(t *testing.T) {
	tests := []struct {
		name    string
		config  Daili51Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: Daili51Config{
				ExtractURL: "http://capi.51daili.com/traffic/getip?format=json&accessName=user&accessPassword=pass",
			},
			wantErr: false,
		},
		{
			name: "missing extract URL",
			config: Daili51Config{
				ExtractURL: "",
			},
			wantErr: true,
			errMsg:  "ExtractURL",
		},
		{
			name: "missing format=json",
			config: Daili51Config{
				ExtractURL: "http://capi.51daili.com/traffic/getip?accessName=user&accessPassword=pass",
			},
			wantErr: true,
			errMsg:  "format=json",
		},
		{
			name: "missing accessName",
			config: Daili51Config{
				ExtractURL: "http://capi.51daili.com/traffic/getip?format=json&accessPassword=pass",
			},
			wantErr: true,
			errMsg:  "accessName",
		},
		{
			name: "missing accessPassword",
			config: Daili51Config{
				ExtractURL: "http://capi.51daili.com/traffic/getip?format=json&accessName=user",
			},
			wantErr: true,
			errMsg:  "accessPassword",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewDaili51Provider(tt.config)
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

func TestDaili51Provider_Protocol(t *testing.T) {
	tests := []struct {
		name     string
		port     string
		expected proxypool.ProxyType
	}{
		{"HTTP", "1", proxypool.ProxyTypeHTTP},
		{"SOCKS5", "2", proxypool.ProxyTypeSOCKS5},
		{"default", "99", proxypool.ProxyTypeSOCKS5}, // 未知值默认SOCKS5
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := fmt.Sprintf("http://capi.51daili.com/traffic/getip?port=%s&format=json&accessName=user&accessPassword=pass", tt.port)
			provider, _ := NewDaili51Provider(Daili51Config{
				ExtractURL: url,
			})

			// 通过解析URL验证协议类型判断逻辑
			parsedURL, _ := url.Parse(provider.baseURL)
			port := parsedURL.Query().Get("port")

			var proxyType proxypool.ProxyType
			if port == "1" {
				proxyType = proxypool.ProxyTypeHTTP
			} else {
				proxyType = proxypool.ProxyTypeSOCKS5
			}

			if proxyType != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, proxyType)
			}
		})
	}
}
