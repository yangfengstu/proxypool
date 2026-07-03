# ProxyPool - 通用HTTP代理池

一个高性能、易用的Go语言代理池库，基于 [req/v3](https://github.com/imroc/req) 构建。

## ✨ 特性

- 🚀 **开箱即用** - 零配置启动，合理的默认值
- 🔌 **可插拔** - 通过Provider接口对接任何代理商
- 🔄 **自动管理** - 自动刷新、过期检测、健康检查
- 📊 **完整监控** - 详细的统计信息和健康度评分
- 🎯 **多种策略** - 轮询、最少使用、随机、按健康度加权
- ⚡ **高性能** - 连接复用、并发拉取、智能选择
- 🔧 **高度可配** - 所有参数都可配置

## 🎯 支持的代理类型

- ✅ SOCKS5
- ✅ HTTP
- ✅ HTTPS

## 📦 安装

```bash
go get github.com/yourusername/proxypool
```

## 🚀 快速开始

### 基础使用

```go
package main

import (
    "fmt"
    "github.com/yourusername/proxypool"
)

func main() {
    // 1. 创建代理提供商（实现Provider接口）
    provider := NewYourProvider()
    
    // 2. 创建代理池
    pool, err := proxypool.New(proxypool.Config{
        Provider:   provider,
        TargetSize: 100,  // 维持100个代理
    })
    if err != nil {
        panic(err)
    }
    defer pool.Close()
    
    // 3. 获取代理客户端（返回的是req.Client）
    client, proxy, err := pool.Get()
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Using proxy: %s\n", proxy.URL())
    
    // 4. 直接使用req/v3的所有方法
    resp, err := client.R().
        SetHeader("User-Agent", "Mozilla/5.0").
        Get("https://api.ipify.org?format=json")
    
    if err != nil {
        // 可选：报告失败
        pool.ReportFailure(proxy, err)
        panic(err)
    }
    
    // 可选：报告成功
    pool.ReportSuccess(proxy, resp.TotalTime())
    
    fmt.Println(resp.String())
}
```

### 实现Provider接口

```go
package main

import (
    "context"
    "time"
    "github.com/yourusername/proxypool"
)

// MyProvider 自定义代理提供商
type MyProvider struct {
    apiKey string
}

func NewMyProvider(apiKey string) *MyProvider {
    return &MyProvider{apiKey: apiKey}
}

func (p *MyProvider) Fetch(ctx context.Context, count int) ([]proxypool.Proxy, error) {
    // 1. 调用你的代理商API
    // 2. 解析返回数据
    // 3. 转换为标准Proxy结构
    
    proxies := make([]proxypool.Proxy, 0, count)
    
    // 示例：从你的API获取代理
    for i := 0; i < count; i++ {
        proxies = append(proxies, proxypool.Proxy{
            Type:      proxypool.ProxyTypeSOCKS5,
            Host:      "proxy.example.com",
            Port:      1080,
            Username:  "user",
            Password:  "pass",
            ExpiredAt: time.Now().Add(60 * time.Minute),  // 重要：设置过期时间
        })
    }
    
    return proxies, nil
}

func (p *MyProvider) Name() string {
    return "my-provider"
}
```

## 🔧 配置选项

### 完整配置示例

```go
pool, _ := proxypool.New(proxypool.Config{
    // ========== 必需配置 ==========
    Provider: myProvider,
    
    // ========== 池大小管理 ==========
    TargetSize:    300,   // 目标池大小
    LowWatermark:  0.7,   // 低于70%触发紧急补充
    HighWatermark: 0.9,   // 预防性维持在90%
    
    // ========== 启动配置 ==========
    StartupBatchSize:   50,  // 启动时每批大小
    StartupConcurrency: 6,   // 启动时并发批次数
    
    // ========== 运行时刷新 ==========
    RefreshWindow: 15 * time.Minute,  // 提前15分钟补充即将过期的代理
    RefreshBatch:  20,                // 运行时每批补充数量
    
    // ========== 选择策略 ==========
    SelectStrategy: proxypool.WeightedByHealth,  // 优先选择健康代理
    
    // ========== HTTP客户端配置 ==========
    EnableKeepAlive:     true,              // 启用连接复用（推荐）
    MaxIdleConns:        100,               // 全局空闲连接数
    MaxIdleConnsPerHost: 10,                // 每个host空闲连接数
    IdleConnTimeout:     90 * time.Second,  // 空闲连接超时
    Timeout:             5 * time.Second,   // 请求超时
    
    // ========== 健康检查 ==========
    HealthCheck:         true,
    HealthCheckURL:      "https://api.ipify.org",
    HealthCheckInterval: 5 * time.Minute,
    MinHealthScore:      0.3,  // 低于此评分剔除
    MaxConsecutiveFails: 5,    // 连续失败5次剔除
    MaxFailRate:         0.8,  // 失败率超过80%剔除
    
    // ========== 日志 ==========
    Logger: myLogger,  // 实现Logger接口
})
```

### 选择策略说明

```go
// 轮询（默认）
SelectStrategy: proxypool.RoundRobin

// 最少使用（负载均衡）
SelectStrategy: proxypool.LeastUsed

// 随机
SelectStrategy: proxypool.Random

// 按健康度加权（推荐）
SelectStrategy: proxypool.WeightedByHealth
```

## 📊 监控统计

```go
// 获取统计信息
stats := pool.Stats()

fmt.Printf("Total: %d\n", stats.Total)
fmt.Printf("Available: %d\n", stats.Available)
fmt.Printf("Utilization: %.2f%%\n", stats.Utilization*100)
fmt.Printf("Success Rate: %.2f%%\n", 
    float64(stats.TotalSuccess)/float64(stats.TotalRequests)*100)

// 查看每个代理的详细统计
for _, ps := range stats.ProxyStats {
    fmt.Printf("Proxy %s: UseCount=%d, SuccessRate=%.2f%%, HealthScore=%.2f\n",
        ps.Proxy.Host,
        ps.UseCount,
        float64(ps.SuccessCount)/float64(ps.UseCount)*100,
        ps.HealthScore)
}
```

## 🎯 使用req/v3的所有功能

代理池返回的是完全配置好的 `req.Client`，你可以使用req/v3的所有方法：

```go
client, proxy, _ := pool.Get()

// 1. 基础请求
resp, _ := client.R().Get("https://api.example.com")

// 2. 设置Headers
resp, _ := client.R().
    SetHeader("User-Agent", "MyApp/1.0").
    SetHeader("Authorization", "Bearer token").
    Get(url)

// 3. POST JSON
resp, _ := client.R().
    SetBodyJsonMarshal(map[string]interface{}{
        "key": "value",
    }).
    Post(url)

// 4. 文件上传
resp, _ := client.R().
    SetFile("file", "/path/to/file").
    Post(url)

// 5. 自动解析JSON响应
var result struct {
    IP string `json:"ip"`
}
resp, _ := client.R().
    SetSuccessResult(&result).
    Get("https://api.ipify.org?format=json")

// 6. 流式下载
resp, _ := client.R().
    SetOutputFile("/path/to/save").
    Get(url)

// 7. 重试（如果需要）
resp, _ := client.R().
    SetRetryCount(3).
    SetRetryBackoffInterval(1*time.Second, 5*time.Second).
    Get(url)

// 更多用法请参考 req/v3 文档
```

## 🔍 进阶用法

### 失败反馈优化

```go
client, proxy, _ := pool.Get()

start := time.Now()
resp, err := client.R().Get(url)
latency := time.Since(start)

if err != nil {
    // 报告失败，帮助池统计
    pool.ReportFailure(proxy, err)
    return err
}

if resp.StatusCode == 407 {
    // 代理认证失败
    pool.ReportFailure(proxy, fmt.Errorf("proxy auth failed"))
    return fmt.Errorf("proxy error")
}

// 报告成功
pool.ReportSuccess(proxy, latency)
```

### 手动刷新

```go
// 手动触发刷新
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

err := pool.Refresh(ctx)
```

### 自定义日志

```go
type MyLogger struct{}

func (l *MyLogger) Printf(format string, v ...interface{}) {
    log.Printf(format, v...)
}

func (l *MyLogger) Errorf(format string, v ...interface{}) {
    log.Printf("ERROR: "+format, v...)
}

pool, _ := proxypool.New(proxypool.Config{
    Provider: myProvider,
    Logger:   &MyLogger{},
})
```

## 📋 工作原理

### 启动流程（1-2秒完成）

```
1. 并发拉取初始代理
   ├─ 批次1: 拉取50个 (1.2秒)
   ├─ 批次2: 拉取50个 (1.3秒)
   └─ ...
2. 添加到池中，立即可用
```

### 运行时维护

```
每1分钟：
├─ 检查水位线
├─ 可用数 < 70% → 紧急补充
├─ 可用数 < 90% → 预防性补充
└─ 即将过期 → 错峰补充

每2分钟：
├─ 计算健康评分
├─ 剔除不健康代理
└─ 触发补充
```

### 健康评分算法

```
健康评分 = 成功率×60% - 超时率×20% + 延迟评分×20%

剔除条件：
- 评分 < 0.3
- 连续失败 >= 5次
- 失败率 > 80%
```

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

## 📄 License

MIT License
