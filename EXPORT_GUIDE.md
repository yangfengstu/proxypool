# 代理信息导出和日志功能指南

## 🎯 功能说明

提供两个核心功能：
1. **代理详情导出** - 获取所有代理的完整信息（包括出口IP）
2. **日志打印代理信息** - 在日志中方便地打印代理信息

## 📋 API 列表

### 1. 获取所有代理详情

```go
// 返回所有代理的详细信息
details := pool.GetAllProxyDetails()

// 每个代理包含：
type ProxyDetail struct {
    // 基础信息
    ProxyURL    string  // 代理URL
    ProxyHost   string  // 代理地址
    ProxyPort   int     // 代理端口
    ProxyType   string  // 代理类型
    
    // 🆕 出口IP信息
    RealExitIP  string  // 真实出口IP
    ExitIPFrom  string  // IP来源（precheck/first_use/unknown）
    
    // 生命周期
    ExpiredAt    time.Time  // 过期时间
    IsExpired    bool       // 是否已过期
    TimeToExpire string     // 距离过期（人类可读）
    
    // 使用统计
    UseCount     int64      // 使用次数
    SuccessCount int64      // 成功次数
    SuccessRate  float64    // 成功率
    
    // 健康状态
    HealthScore  float64    // 健康评分
    AvgLatency   string     // 平均延迟
    
    // ... 更多字段
}
```

### 2. 导出为JSON

```go
// 导出所有代理信息为JSON字符串
jsonData, err := pool.ExportProxyDetailsJSON()
if err != nil {
    log.Printf("Export error: %v", err)
} else {
    // 保存到文件
    ioutil.WriteFile("proxies.json", []byte(jsonData), 0644)
}
```

### 3. 根据地址查找代理

```go
// 根据代理地址查找详情
detail := pool.GetProxyByHost("192.168.1.1", 1080)
if detail != nil {
    fmt.Printf("Exit IP: %s\n", detail.RealExitIP)
}
```

### 4. 获取代理信息（用于日志）

```go
_, proxy, _ := pool.Get()

// 获取代理信息
info := pool.GetProxyInfo(proxy)

// 在日志中打印
log.Printf("Request started %s", info)
// 输出: Request started proxy=192.168.1.1:1080 type=socks5 exit_ip=1.2.3.4 use=10 health=0.85
```

### 5. 安全URL（隐藏密码）

```go
// 原始URL（包含密码）
proxy.URL()  // socks5://user:password123@host:1080

// 安全URL（隐藏密码）
proxy.SafeURL()  // socks5://user:pa****23@host:1080
```

## 🎯 使用场景

### 场景1：监控面板数据源

```go
// 为监控面板提供数据
func getProxyMetrics() []ProxyMetric {
    details := pool.GetAllProxyDetails()
    metrics := make([]ProxyMetric, len(details))
    
    for i, d := range details {
        metrics[i] = ProxyMetric{
            Host:       d.ProxyHost,
            ExitIP:     d.RealExitIP,
            Health:     d.HealthScore,
            SuccessRate: d.SuccessRate,
            Status:     getStatus(d),
        }
    }
    
    return metrics
}
```

### 场景2：导出为文件

```go
// 定期导出代理信息
func exportToFile(pool *proxypool.Pool) {
    jsonData, err := pool.ExportProxyDetailsJSON()
    if err != nil {
        log.Printf("Export failed: %v", err)
        return
    }
    
    filename := fmt.Sprintf("proxies_%s.json", 
        time.Now().Format("20060102_150405"))
    
    if err := ioutil.WriteFile(filename, []byte(jsonData), 0644); err != nil {
        log.Printf("Write file failed: %v", err)
        return
    }
    
    log.Printf("Exported to %s", filename)
}

// 定时导出
go func() {
    ticker := time.NewTicker(1 * time.Hour)
    for range ticker.C {
        exportToFile(pool)
    }
}()
```

### 场景3：HTTP API接口

```go
// 提供HTTP API查询代理信息
http.HandleFunc("/api/proxies", func(w http.ResponseWriter, r *http.Request) {
    details := pool.GetAllProxyDetails()
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "total": len(details),
        "proxies": details,
    })
})

// 查询单个代理
http.HandleFunc("/api/proxy", func(w http.ResponseWriter, r *http.Request) {
    host := r.URL.Query().Get("host")
    port, _ := strconv.Atoi(r.URL.Query().Get("port"))
    
    detail := pool.GetProxyByHost(host, port)
    if detail == nil {
        http.NotFound(w, r)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(detail)
})
```

### 场景4：结构化日志

```go
// 使用logrus等结构化日志库
import "github.com/sirupsen/logrus"

_, proxy, _ := pool.Get()
info := pool.GetProxyInfo(proxy)

logrus.WithFields(logrus.Fields{
    "proxy_host":    info.Host,
    "proxy_port":    info.Port,
    "proxy_type":    info.Type,
    "exit_ip":       info.ExitIP,
    "health_score":  info.Health,
    "use_count":     info.UseCount,
}).Info("Request completed")
```

### 场景5：出口IP追踪

```go
// 检查出口IP是否变化
func checkExitIP(pool *proxypool.Pool) {
    details := pool.GetAllProxyDetails()
    
    for _, d := range details {
        if d.RealExitIP == "" {
            log.Printf("⚠️  Proxy %s:%d has no exit IP detected",
                d.ProxyHost, d.ProxyPort)
            continue
        }
        
        // 检查出口IP是否与代理地址一致
        if d.RealExitIP != d.ProxyHost {
            log.Printf("ℹ️  Proxy %s:%d exit IP: %s (different from proxy host)",
                d.ProxyHost, d.ProxyPort, d.RealExitIP)
        }
    }
}
```

### 场景6：健康报告

```go
// 生成健康报告
func generateHealthReport(pool *proxypool.Pool) {
    details := pool.GetAllProxyDetails()
    
    var healthy, unhealthy, expired int
    var totalSuccessRate float64
    
    for _, d := range details {
        if d.IsExpired {
            expired++
        } else if d.HealthScore >= 0.7 {
            healthy++
        } else {
            unhealthy++
        }
        totalSuccessRate += d.SuccessRate
    }
    
    avgSuccessRate := totalSuccessRate / float64(len(details))
    
    fmt.Printf("📊 Proxy Health Report\n")
    fmt.Printf("   Total: %d\n", len(details))
    fmt.Printf("   Healthy: %d (%.1f%%)\n", healthy, 
        float64(healthy)/float64(len(details))*100)
    fmt.Printf("   Unhealthy: %d (%.1f%%)\n", unhealthy,
        float64(unhealthy)/float64(len(details))*100)
    fmt.Printf("   Expired: %d (%.1f%%)\n", expired,
        float64(expired)/float64(len(details))*100)
    fmt.Printf("   Avg Success Rate: %.1f%%\n", avgSuccessRate)
}
```

## 📊 JSON导出格式

```json
[
  {
    "proxy_url": "socks5://user:pass@192.168.1.1:1080",
    "proxy_host": "192.168.1.1",
    "proxy_port": 1080,
    "proxy_type": "socks5",
    "username": "user",
    "region": "上海",
    "isp": "电信",
    "real_exit_ip": "1.2.3.4",
    "exit_ip_from": "precheck",
    "expired_at": "2024-07-03T15:30:00Z",
    "is_expired": false,
    "time_to_expire": "45m30s",
    "use_count": 25,
    "success_count": 23,
    "fail_count": 2,
    "timeout_count": 1,
    "success_rate": 92.0,
    "health_score": 0.85,
    "consecutive_fails": 0,
    "avg_latency": "523ms",
    "last_used": "2024-07-03T14:50:00Z",
    "precheck_latency": "456ms",
    "precheck_time": "2024-07-03T14:30:00Z"
  }
]
```

## 💡 最佳实践

### 1. 定期导出备份
```go
// 每小时导出一次
ticker := time.NewTicker(1 * time.Hour)
go func() {
    for range ticker.C {
        exportToFile(pool)
    }
}()
```

### 2. 日志中包含代理信息
```go
// 请求开始
info := pool.GetProxyInfo(proxy)
log.Printf("[START] %s url=%s", info, targetURL)

// 请求结束
log.Printf("[END] %s status=%d latency=%v", info, resp.StatusCode, latency)
```

### 3. 监控出口IP变化
```go
// 缓存出口IP
exitIPCache := make(map[string]string)

info := pool.GetProxyInfo(proxy)
key := fmt.Sprintf("%s:%d", info.Host, info.Port)

if cachedIP, exists := exitIPCache[key]; exists {
    if cachedIP != info.ExitIP && info.ExitIP != "" {
        log.Printf("⚠️  Exit IP changed: %s -> %s", cachedIP, info.ExitIP)
    }
}
exitIPCache[key] = info.ExitIP
```

## 🧪 测试

```bash
# 运行导出功能测试
go test -v -run TestExport

# 运行导出示例
go run examples/export/main.go
```

## 📝 注意事项

1. **出口IP来源**
   - `precheck`: 来自预检（推荐启用PreCheck）
   - `first_use`: 来自首次使用时记录
   - `unknown`: 未检测到

2. **SafeURL** - 默认隐藏密码中间部分

3. **JSON导出** - 包含完整信息，注意保护敏感数据

4. **性能** - `GetAllProxyDetails()`会锁定池，建议异步调用

---

**导出功能让代理池更透明、更易监控！**
