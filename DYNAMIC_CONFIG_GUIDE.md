# 动态配置功能指南

## 🔧 功能说明

动态配置允许你在代理池运行时实时调整各项参数，无需重启池子。

### ✨ 支持的配置项

#### 1. 池大小管理
- `TargetSize` - 目标池大小
- `LowWatermark` - 低水位线（触发紧急补充）
- `HighWatermark` - 高水位线（预防性维持）

#### 2. 监控开关
- `AutoRefresh` - 是否自动刷新
- `AutoPrune` - 是否自动剔除不健康代理

#### 3. 时间间隔
- `MonitorInterval` - 水位线检查间隔
- `PruneInterval` - 不健康剔除间隔

#### 4. 健康阈值
- `MinHealthScore` - 最低健康评分
- `MaxConsecutiveFails` - 最大连续失败次数
- `MaxFailRate` - 最大失败率

#### 5. 预检配置
- `PreCheckEnabled` - 是否启用预检
- `PreCheckMaxLatency` - 预检最大延迟
- `PreCheckConcurrency` - 预检并发数

## 🚀 使用方法

### 基础用法

```go
pool, _ := proxypool.New(proxypool.Config{
    Provider:   provider,
    TargetSize: 100,
})

// 扩大池子
newSize := 200
pool.UpdateConfig(proxypool.UpdateConfig{
    TargetSize: &newSize,
})

// 调整水位线
lowWater := 0.6
highWater := 0.85
pool.UpdateConfig(proxypool.UpdateConfig{
    LowWatermark:  &lowWater,
    HighWatermark: &highWater,
})
```

### 开关控制

```go
// 关闭自动刷新（临时停止拉取新代理）
autoRefresh := false
pool.UpdateConfig(proxypool.UpdateConfig{
    AutoRefresh: &autoRefresh,
})

// 关闭自动剔除（保留所有代理）
autoPrune := false
pool.UpdateConfig(proxypool.UpdateConfig{
    AutoPrune: &autoPrune,
})

// 重新启用
autoRefresh = true
autoPrune = true
pool.UpdateConfig(proxypool.UpdateConfig{
    AutoRefresh: &autoRefresh,
    AutoPrune:   &autoPrune,
})
```

### 调整健康阈值

```go
// 更严格的健康要求
minScore := 0.5  // 从0.3提升到0.5
maxFails := 3    // 从5降低到3
maxRate := 0.6   // 从0.8降低到0.6

pool.UpdateConfig(proxypool.UpdateConfig{
    MinHealthScore:      &minScore,
    MaxConsecutiveFails: &maxFails,
    MaxFailRate:         &maxRate,
})
```

### 调整预检配置

```go
// 运行时启用预检
enabled := true
maxLatency := 2 * time.Second
concurrency := 20

pool.UpdateConfig(proxypool.UpdateConfig{
    PreCheckEnabled:     &enabled,
    PreCheckMaxLatency:  &maxLatency,
    PreCheckConcurrency: &concurrency,
})

// 运行时关闭预检
enabled = false
pool.UpdateConfig(proxypool.UpdateConfig{
    PreCheckEnabled: &enabled,
})
```

### 调整检查频率

```go
// 降低检查频率（节省CPU）
monitorInterval := 2 * time.Minute
pruneInterval := 5 * time.Minute

pool.UpdateConfig(proxypool.UpdateConfig{
    MonitorInterval: &monitorInterval,
    PruneInterval:   &pruneInterval,
})
```

## 📋 查看当前配置

```go
cfg := pool.GetCurrentConfig()

fmt.Printf("TargetSize: %d\n", cfg.TargetSize)
fmt.Printf("AutoRefresh: %v\n", cfg.AutoRefresh)
fmt.Printf("AutoPrune: %v\n", cfg.AutoPrune)
fmt.Printf("MinHealthScore: %.2f\n", cfg.MinHealthScore)
// ... 更多配置
```

## 🎯 实际应用场景

### 场景1：高峰期扩容

```go
// 业务高峰期，扩大池子
if isHighTraffic() {
    newSize := 500
    pool.UpdateConfig(proxypool.UpdateConfig{
        TargetSize: &newSize,
    })
}

// 低峰期，缩小池子
if isLowTraffic() {
    newSize := 100
    pool.UpdateConfig(proxypool.UpdateConfig{
        TargetSize: &newSize,
    })
}
```

### 场景2：临时暂停刷新

```go
// 代理余额不足，临时关闭自动刷新
if balance < 10 {
    autoRefresh := false
    pool.UpdateConfig(proxypool.UpdateConfig{
        AutoRefresh: &autoRefresh,
    })
    alert("Proxy balance low, auto-refresh disabled")
}
```

### 场景3：质量控制

```go
// 发现质量下降，提高健康要求
if successRate < 0.8 {
    minScore := 0.6
    maxFails := 2
    pool.UpdateConfig(proxypool.UpdateConfig{
        MinHealthScore:      &minScore,
        MaxConsecutiveFails: &maxFails,
    })
}
```

### 场景4：配置热重载

```go
// 从配置文件/数据库读取最新配置
func reloadConfig(pool *proxypool.Pool) {
    cfg := loadConfigFromFile()
    
    pool.UpdateConfig(proxypool.UpdateConfig{
        TargetSize:          &cfg.TargetSize,
        LowWatermark:        &cfg.LowWatermark,
        HighWatermark:       &cfg.HighWatermark,
        MinHealthScore:      &cfg.MinHealthScore,
        MaxConsecutiveFails: &cfg.MaxConsecutiveFails,
    })
    
    log.Println("Config reloaded successfully")
}

// 定期重载
go func() {
    ticker := time.NewTicker(5 * time.Minute)
    for range ticker.C {
        reloadConfig(pool)
    }
}()
```

## ⚙️ 配置生效时间

| 配置项 | 生效时间 | 说明 |
|-------|---------|------|
| TargetSize | 立即 | 下次检查时生效 |
| LowWatermark | 立即 | 下次检查时生效 |
| HighWatermark | 立即 | 下次检查时生效 |
| AutoRefresh | 立即 | 下次检查时判断 |
| AutoPrune | 立即 | 下次剔除时判断 |
| MonitorInterval | 下个周期 | 当前周期结束后生效 |
| PruneInterval | 下个周期 | 当前周期结束后生效 |
| MinHealthScore | 立即 | 下次剔除时生效 |
| MaxConsecutiveFails | 立即 | 立即判断 |
| PreCheckEnabled | 立即 | 下次拉取时生效 |
| PreCheckMaxLatency | 立即 | 下次预检时生效 |
| PreCheckConcurrency | 立即 | 下次预检时生效 |

## 💡 最佳实践

### 1. 渐进式调整
```go
// ❌ 不要一次性大幅调整
newSize := 1000  // 从100直接跳到1000
pool.UpdateConfig(proxypool.UpdateConfig{TargetSize: &newSize})

// ✅ 逐步调整
newSize := 200   // 从100到200
pool.UpdateConfig(proxypool.UpdateConfig{TargetSize: &newSize})
time.Sleep(1 * time.Minute)
newSize = 300    // 再到300
pool.UpdateConfig(proxypool.UpdateConfig{TargetSize: &newSize})
```

### 2. 监控配置变更
```go
// 记录配置变更
func updateWithLog(pool *proxypool.Pool, update proxypool.UpdateConfig) {
    before := pool.GetCurrentConfig()
    pool.UpdateConfig(update)
    after := pool.GetCurrentConfig()
    
    log.Printf("Config changed: TargetSize %d -> %d", 
        before.TargetSize, after.TargetSize)
}
```

### 3. 配置验证
```go
// 验证配置合理性
func validateAndUpdate(pool *proxypool.Pool, newSize int) error {
    if newSize < 10 || newSize > 1000 {
        return fmt.Errorf("invalid TargetSize: %d", newSize)
    }
    
    pool.UpdateConfig(proxypool.UpdateConfig{
        TargetSize: &newSize,
    })
    return nil
}
```

## 🔍 调试技巧

### 查看配置差异
```go
before := pool.GetCurrentConfig()

// 更新配置
pool.UpdateConfig(update)

after := pool.GetCurrentConfig()

// 比较差异
if before.TargetSize != after.TargetSize {
    log.Printf("TargetSize changed: %d -> %d", before.TargetSize, after.TargetSize)
}
```

### 导出配置
```go
func exportConfig(pool *proxypool.Pool) map[string]interface{} {
    cfg := pool.GetCurrentConfig()
    return map[string]interface{}{
        "target_size":     cfg.TargetSize,
        "auto_refresh":    cfg.AutoRefresh,
        "auto_prune":      cfg.AutoPrune,
        "min_health_score": cfg.MinHealthScore,
        // ... 更多
    }
}

// 保存到文件
data, _ := json.MarshalIndent(exportConfig(pool), "", "  ")
ioutil.WriteFile("pool_config.json", data, 0644)
```

## ⚠️ 注意事项

1. **线程安全** - 所有更新操作都是线程安全的
2. **生效时机** - 大部分配置立即生效，间隔时间需要等到下个周期
3. **合理范围** - 建议在合理范围内调整，避免极端值
4. **监控影响** - 配置变更后注意观察池的行为
5. **兼容性** - UpdateConfig使用指针，只更新非nil的字段

## 🧪 测试

```bash
# 运行动态配置测试
go test -v -run TestDynamicConfig

# 运行动态配置示例
go run examples/dynamic_config/main.go
```

## 📊 配置建议

### 高QPS场景
```go
pool.UpdateConfig(proxypool.UpdateConfig{
    TargetSize:      ptr(500),
    LowWatermark:    ptr(0.8),
    HighWatermark:   ptr(0.95),
    MonitorInterval: ptr(30 * time.Second),
})
```

### 低QPS场景
```go
pool.UpdateConfig(proxypool.UpdateConfig{
    TargetSize:      ptr(50),
    LowWatermark:    ptr(0.6),
    HighWatermark:   ptr(0.8),
    MonitorInterval: ptr(2 * time.Minute),
})
```

### 严格质量控制
```go
pool.UpdateConfig(proxypool.UpdateConfig{
    MinHealthScore:      ptr(0.6),
    MaxConsecutiveFails: ptr(2),
    MaxFailRate:         ptr(0.5),
    PreCheckEnabled:     ptr(true),
    PreCheckMaxLatency:  ptr(2 * time.Second),
})
```

---

**动态配置让代理池更灵活、更智能！**
