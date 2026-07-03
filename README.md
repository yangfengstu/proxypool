# ProxyPool

通用HTTP代理池 - 支持任意代理商，基于 [req/v3](https://github.com/imroc/req) 构建

[![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.21-blue)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

## ✨ 核心特性

- 🚀 **极简配置** - 只需提取链接，无需复杂参数
- 🔌 **可插拔** - 通过Provider接口对接任何代理商
- 🔄 **自动管理** - 自动刷新、过期检测、健康检查
- 🧪 **代理预检** - 进入池前并发检测，确保100%可用
- 📊 **完整监控** - 详细的统计信息和健康度评分
- 🎯 **多种策略** - 轮询、最少使用、随机、按健康度加权
- ⚡ **高性能** - 连接复用、并发拉取、智能选择
- 🔧 **高度可配** - 20+配置参数，所有行为可控

## 📦 已集成的代理商

- ✅ **IP赞** (ipzan.com) - SOCKS5/HTTP
- ✅ **51代理** (51daili.com) - SOCKS5/HTTP
- ✅ 更多代理商可通过实现Provider接口集成

## 🚀 快速开始

### 安装

```bash
go get github.com/yourusername/proxypool
```

### 使用IP赞

```go
package main

import (
    "github.com/yourusername/proxypool"
    "github.com/yourusername/proxypool/providers"
)

func main() {
    // 1. 创建Provider（只需提取链接）
    provider, _ := providers.NewIPZanProvider(providers.IPZanConfig{
        ExtractURL: "你的IP赞完整提取链接",
    })
    
    // 2. 创建代理池
    pool, _ := proxypool.New(proxypool.Config{
        Provider:   provider,
        TargetSize: 100,
        
        // 🆕 启用预检（推荐）
        PreCheck: proxypool.PreCheckConfig{
            Enabled:    true,
            MaxLatency: 3 * time.Second,
        },
    })
    defer pool.Close()
    
    // 3. 获取代理并使用
    client, proxy, _ := pool.Get()
    resp, _ := client.R().Get("https://api.example.com")
    
    // 4. 反馈结果（可选，帮助池优化）
    pool.ReportSuccess(proxy, resp.TotalTime())
}
```

### 使用51代理

```go
provider, _ := providers.NewDaili51Provider(providers.Daili51Config{
    ExtractURL: "你的51代理完整提取链接",
})

pool, _ := proxypool.New(proxypool.Config{
    Provider:   provider,
    TargetSize: 100,
})
```

## 🆕 代理预检功能

**确保进入池子的代理100%可用！**

```go
PreCheck: proxypool.PreCheckConfig{
    Enabled:         true,                // 启用预检
    Timeout:         5 * time.Second,     // 预检超时
    MaxLatency:      3 * time.Second,     // 最大允许延迟
    Concurrency:     10,                  // 并发worker数
    RequireRealIP:   true,                // 要求检测到真实IP
    MinSuccessCount: 1,                   // 最少成功次数
}
```

**预检会自动：**
- ✅ 并发检测每个代理的连通性
- ✅ 获取真实出口IP地址
- ✅ 测量实际响应延迟
- ✅ 剔除响应过慢的代理

详细说明见：[PRECHECK_GUIDE.md](PRECHECK_GUIDE.md)

## 🔧 完整配置

```go
pool, _ := proxypool.New(proxypool.Config{
    // 必需
    Provider: provider,
    
    // 池大小管理
    TargetSize:    300,   // 目标池大小
    LowWatermark:  0.7,   // 低于70%触发紧急补充
    HighWatermark: 0.9,   // 预防性维持在90%
    
    // 选择策略
    SelectStrategy: proxypool.WeightedByHealth,  // 优先选择健康代理
    
    // HTTP客户端
    EnableKeepAlive:     true,              // 启用连接复用
    MaxIdleConns:        100,               // 全局空闲连接数
    MaxIdleConnsPerHost: 10,                // 每个host空闲连接数
    IdleConnTimeout:     90 * time.Second,  // 空闲连接超时
    
    // 预检（推荐）
    PreCheck: proxypool.PreCheckConfig{
        Enabled:    true,
        MaxLatency: 3 * time.Second,
    },
    
    // 健康检查
    HealthCheck:         true,
    HealthCheckURL:      "https://api.ipify.org",
    MinHealthScore:      0.3,
    MaxConsecutiveFails: 5,
})
```

## 📊 使用req/v3的所有功能

返回的是完全配置好的 `req.Client`，支持所有req/v3方法：

```go
client, _, _ := pool.Get()

// 基础请求
client.R().Get(url)

// 设置Headers
client.R().
    SetHeader("User-Agent", "MyApp").
    SetHeader("Authorization", "Bearer token").
    Get(url)

// POST JSON
client.R().
    SetBodyJsonMarshal(map[string]interface{}{
        "key": "value",
    }).
    Post(url)

// 文件上传
client.R().
    SetFile("file", "/path/to/file").
    Post(url)

// 自动解析JSON
var result struct {
    IP string `json:"ip"`
}
client.R().
    SetSuccessResult(&result).
    Get("https://api.ipify.org?format=json")

// 更多用法请参考 req/v3 文档
```

## 📈 监控统计

```go
stats := pool.Stats()

fmt.Printf("Total: %d\n", stats.Total)
fmt.Printf("Available: %d (%.2f%%)\n", stats.Available, stats.Utilization*100)
fmt.Printf("Success Rate: %.2f%%\n", 
    float64(stats.TotalSuccess)/float64(stats.TotalRequests)*100)

// 查看预检结果
for _, ps := range stats.ProxyStats {
    latency := ps.Proxy.Metadata["precheck_latency"]
    realIP := ps.Proxy.Metadata["precheck_real_ip"]
    fmt.Printf("Proxy: %s, Latency: %s, IP: %s\n", 
        ps.Proxy.Host, latency, realIP)
}
```

## 🧪 测试

```bash
# 单元测试
go test -v

# Live测试（需要真实提取链接）
go run cmd/live_test/main.go \
  -provider=ipzan \
  -url='你的提取链接' \
  -count=5

# 预检示例
go run examples/precheck/main.go
```

## 📚 文档

- [PRECHECK_GUIDE.md](PRECHECK_GUIDE.md) - 代理预检功能指南
- [LIVE_TEST.md](LIVE_TEST.md) - Live测试指南
- [HOW_TRANSPARENT_WORKS.md](HOW_TRANSPARENT_WORKS.md) - 技术说明
- [PROJECT_SUMMARY.md](PROJECT_SUMMARY.md) - 项目总结

## 🎯 自定义Provider

实现Provider接口即可对接任何代理商：

```go
type MyProvider struct {
    extractURL string
}

func (p *MyProvider) Fetch(ctx context.Context, count int) ([]proxypool.Proxy, error) {
    // 1. 调用你的代理商API
    // 2. 解析响应
    // 3. 返回Proxy切片（包含ExpiredAt字段）
}

func (p *MyProvider) Name() string {
    return "my-provider"
}
```

## 🌟 设计亮点

### 1. 极简配置
只需提取链接，无需解析参数：
```go
provider, _ := providers.NewIPZanProvider(providers.IPZanConfig{
    ExtractURL: "完整提取链接",  // 一行配置！
})
```

### 2. 零成本抽象
直接返回`*req.Client`，不包装：
- req/v3所有方法天然可用
- 无性能损失
- 未来兼容

### 3. 智能管理
- 启动：1-2秒并发打满池子
- 运行：水位线自动维持（70%-90%）
- 预检：并发检测，确保质量
- 剔除：健康评分<0.3自动移除

## 📄 License

MIT License

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！
