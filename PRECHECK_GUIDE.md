# 代理预检功能使用指南

## 📋 功能说明

预检功能会在代理进入池子之前，并发检测每个代理的：
1. ✅ **连通性** - 能否正常连接
2. ✅ **真实出口IP** - 检测真实的出口IP地址
3. ✅ **响应延迟** - 测量实际延迟
4. ✅ **质量筛选** - 剔除响应过慢的代理

## 🚀 快速启用

```go
pool, _ := proxypool.New(proxypool.Config{
    Provider:   provider,
    TargetSize: 100,
    
    // ✨ 启用预检
    PreCheck: proxypool.PreCheckConfig{
        Enabled:    true,
        MaxLatency: 3 * time.Second, // 超过3秒剔除
    },
})
```

## 🔧 完整配置

```go
PreCheck: proxypool.PreCheckConfig{
    // 基础配置
    Enabled:         true,                // 是否启用，默认false
    Timeout:         5 * time.Second,     // 单个检测超时
    MaxLatency:      3 * time.Second,     // 最大允许延迟
    Concurrency:     10,                  // 并发worker数
    
    // 检测要求
    RequireRealIP:   true,                // 是否要求获取真实IP
    MinSuccessCount: 1,                   // 最少成功次数
    
    // 检测URL列表（可选，默认使用内置列表）
    CheckURLs: []string{
        "https://api.ipify.org?format=json",
        "https://ifconfig.me/ip",
        "https://api.myip.com",
    },
}
```

## 📊 内置检测URL

预检默认使用以下URL（按顺序尝试）：

1. **https://api.ipify.org?format=json** - 主要检测
2. **https://ifconfig.me/ip** - 备用1
3. **https://api.myip.com** - 备用2
4. **https://checkip.amazonaws.com** - 备用3
5. **https://icanhazip.com** - 备用4

达到 `MinSuccessCount` 次成功即通过。

## 🎯 预检流程

```
1. 拉取代理 (Provider.Fetch)
   ↓
2. 并发预检 (10个worker)
   ├─ Worker 1: 检测代理1,11,21...
   ├─ Worker 2: 检测代理2,12,22...
   └─ ...
   ↓
3. 质量筛选
   ├─ 超时 → 剔除
   ├─ 延迟>MaxLatency → 剔除
   ├─ 无法获取IP → 剔除（如果RequireRealIP=true）
   └─ 通过 → 加入池子
   ↓
4. 保存预检结果到Metadata
   ├─ precheck_latency: "523ms"
   ├─ precheck_real_ip: "218.95.39.77"
   └─ precheck_at: "2024-07-03T14:26:26Z"
```

## 📈 性能影响

### 启动时间
- **不启用预检**: 1-2秒（仅拉取）
- **启用预检**: 3-6秒（拉取+并发检测）

### 代理质量
- **不启用**: 可能有10-20%的代理不可用
- **启用**: 进入池子的代理100%可用

### 建议
- **生产环境**: 建议启用（质量优先）
- **开发测试**: 可不启用（速度优先）

## 🔍 查看预检结果

```go
stats := pool.Stats()

for _, ps := range stats.ProxyStats {
    // 查看预检延迟
    latency := ps.Proxy.Metadata["precheck_latency"]
    
    // 查看真实出口IP
    realIP := ps.Proxy.Metadata["precheck_real_ip"]
    
    // 查看检测时间
    checkedAt := ps.Proxy.Metadata["precheck_at"]
    
    fmt.Printf("Proxy: %s, Latency: %s, Real IP: %s\n", 
        ps.Proxy.Host, latency, realIP)
}
```

## 💡 使用建议

### 1. 根据场景调整并发数
```go
// 小批量（<50个）
Concurrency: 5

// 中批量（50-100个）
Concurrency: 10

// 大批量（>100个）
Concurrency: 20
```

### 2. 根据需求调整延迟阈值
```go
// 严格要求（API调用）
MaxLatency: 2 * time.Second

// 一般要求（爬虫）
MaxLatency: 3 * time.Second

// 宽松要求（批量任务）
MaxLatency: 5 * time.Second
```

### 3. 自定义检测URL
```go
PreCheck: proxypool.PreCheckConfig{
    Enabled: true,
    // 使用自己的检测服务
    CheckURLs: []string{
        "https://your-check-service.com/ip",
    },
}
```

## 🧪 测试预检功能

```bash
cd /Users/leo/Workspace/apps/proxypool

# 运行预检测试
go test -v -run TestPreCheck

# 运行预检示例
go run examples/precheck/main.go
```

## ⚙️ 配置示例

### 示例1：严格模式（质量优先）
```go
PreCheck: proxypool.PreCheckConfig{
    Enabled:         true,
    MaxLatency:      2 * time.Second,  // 2秒阈值
    RequireRealIP:   true,             // 必须有IP
    MinSuccessCount: 2,                // 至少成功2次
    Concurrency:     15,               // 高并发
}
```

### 示例2：平衡模式（推荐）
```go
PreCheck: proxypool.PreCheckConfig{
    Enabled:         true,
    MaxLatency:      3 * time.Second,  // 3秒阈值
    RequireRealIP:   true,             // 必须有IP
    MinSuccessCount: 1,                // 至少成功1次
    Concurrency:     10,               // 中等并发
}
```

### 示例3：宽松模式（速度优先）
```go
PreCheck: proxypool.PreCheckConfig{
    Enabled:         true,
    MaxLatency:      5 * time.Second,  // 5秒阈值
    RequireRealIP:   false,            // 不要求IP
    MinSuccessCount: 1,                // 至少成功1次
    Concurrency:     5,                // 低并发
}
```

## 📊 日志输出示例

```
2024/07/03 14:26:26 Initializing proxy pool: target=100, batchSize=20, concurrency=5
2024/07/03 14:26:27 Pool initialized: 100 proxies in 1.2s (errors: 0)
2024/07/03 14:26:27 Pre-checking 100 proxies with 10 workers...
2024/07/03 14:26:28   ✓ 218.95.39.77:11638 - OK (latency: 523ms, IP: 218.95.39.77)
2024/07/03 14:26:28   ✓ 221.229.220.22:38792 - OK (latency: 612ms, IP: 221.229.220.22)
2024/07/03 14:26:29   ✗ 192.168.1.1:1080 - Too slow: 3.5s (max: 3s)
2024/07/03 14:26:29   ✗ 10.0.0.1:1080 - Failed: timeout
2024/07/03 14:26:32 Pre-check completed in 5.2s: 87/100 passed (timeout: 8, slow: 3, no_ip: 2, other: 0)
```

## ❓ 常见问题

### Q1: 预检太慢怎么办？
**A:** 增加 `Concurrency` 或减少 `Timeout`

### Q2: 预检通过率太低？
**A:** 放宽 `MaxLatency` 或设置 `RequireRealIP: false`

### Q3: 如何跳过预检？
**A:** 设置 `PreCheck.Enabled: false`

### Q4: 预检会消耗代理流量吗？
**A:** 会，每个代理检测1-5次（取决于MinSuccessCount和失败重试）

## 🎯 最佳实践

1. ✅ **生产环境始终启用预检**
2. ✅ **根据业务调整MaxLatency**
3. ✅ **监控预检通过率**
4. ✅ **定期检查预检日志**
5. ✅ **考虑使用自己的检测服务**
