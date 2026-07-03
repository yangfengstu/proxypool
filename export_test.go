package proxypool

import (
	"testing"
)

func TestExport_GetAllProxyDetails(t *testing.T) {
	pool, err := New(Config{
		Provider:   NewExampleProvider(),
		TargetSize: 5,
		Logger:     &testLogger{t: t},
	})
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	details := pool.GetAllProxyDetails()
	if len(details) != 5 {
		t.Errorf("Expected 5 proxy details, got %d", len(details))
	}

	// 验证字段
	for i, detail := range details {
		if detail.ProxyHost == "" {
			t.Errorf("Proxy %d: empty host", i)
		}
		if detail.ProxyPort == 0 {
			t.Errorf("Proxy %d: invalid port", i)
		}
		if detail.ProxyType == "" {
			t.Errorf("Proxy %d: empty type", i)
		}
		if detail.ProxyURL == "" {
			t.Errorf("Proxy %d: empty URL", i)
		}
	}
}

func TestExport_ExportJSON(t *testing.T) {
	pool, err := New(Config{
		Provider:   NewExampleProvider(),
		TargetSize: 3,
		Logger:     &testLogger{t: t},
	})
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	jsonData, err := pool.ExportProxyDetailsJSON()
	if err != nil {
		t.Fatalf("Export JSON failed: %v", err)
	}

	if len(jsonData) == 0 {
		t.Error("JSON export is empty")
	}

	t.Logf("Exported JSON length: %d bytes", len(jsonData))
}

func TestExport_GetProxyByHost(t *testing.T) {
	pool, err := New(Config{
		Provider:   NewExampleProvider(),
		TargetSize: 5,
		Logger:     &testLogger{t: t},
	})
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	details := pool.GetAllProxyDetails()
	if len(details) == 0 {
		t.Fatal("No proxies available")
	}

	first := details[0]
	found := pool.GetProxyByHost(first.ProxyHost, first.ProxyPort)
	if found == nil {
		t.Fatal("Proxy not found")
	}

	if found.ProxyHost != first.ProxyHost {
		t.Errorf("Expected host %s, got %s", first.ProxyHost, found.ProxyHost)
	}
	if found.ProxyPort != first.ProxyPort {
		t.Errorf("Expected port %d, got %d", first.ProxyPort, found.ProxyPort)
	}
}

func TestExport_GetProxyInfo(t *testing.T) {
	pool, err := New(Config{
		Provider:   NewExampleProvider(),
		TargetSize: 3,
		Logger:     &testLogger{t: t},
	})
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	_, proxy, err := pool.Get()
	if err != nil {
		t.Fatalf("Failed to get proxy: %v", err)
	}

	info := pool.GetProxyInfo(proxy)
	if info.Host == "" {
		t.Error("ProxyInfo has empty host")
	}
	if info.Port == 0 {
		t.Error("ProxyInfo has invalid port")
	}
	if info.Type == "" {
		t.Error("ProxyInfo has empty type")
	}

	// 测试String方法
	infoStr := info.String()
	if infoStr == "" {
		t.Error("ProxyInfo.String() is empty")
	}
	t.Logf("ProxyInfo: %s", infoStr)
}

func TestProxy_SafeURL(t *testing.T) {
	proxy := Proxy{
		Type:     ProxyTypeSOCKS5,
		Host:     "127.0.0.1",
		Port:     1080,
		Username: "user",
		Password: "password123",
	}

	safeURL := proxy.SafeURL()
	if !contains(safeURL, "user") {
		t.Error("SafeURL should contain username")
	}
	if contains(safeURL, "password123") {
		t.Error("SafeURL should not contain full password")
	}
	if !contains(safeURL, "****") {
		t.Error("SafeURL should mask password")
	}
	t.Logf("SafeURL: %s", safeURL)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s != "" && substr != "" &&
		(s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
