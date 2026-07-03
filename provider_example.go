package proxypool

import (
	"context"
	"fmt"
	"time"
)

// ExampleProvider 示例提供商（用于演示和测试）
type ExampleProvider struct {
	name string
}

// NewExampleProvider 创建示例提供商
func NewExampleProvider() *ExampleProvider {
	return &ExampleProvider{name: "example"}
}

// Fetch 实现Provider接口（返回模拟数据）
func (p *ExampleProvider) Fetch(ctx context.Context, count int) ([]Proxy, error) {
	proxies := make([]Proxy, 0, count)

	for i := 0; i < count; i++ {
		proxies = append(proxies, Proxy{
			Type:      ProxyTypeSOCKS5,
			Host:      fmt.Sprintf("127.0.0.%d", i+1),
			Port:      1080 + i,
			Username:  "user",
			Password:  "pass",
			ExpiredAt: time.Now().Add(60 * time.Minute),
			Region:    "CN",
		})
	}

	return proxies, nil
}

// Name 实现Provider接口
func (p *ExampleProvider) Name() string {
	return p.name
}
