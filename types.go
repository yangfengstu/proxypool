package proxypool

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrNoProvider        = errors.New("proxypool: provider is required")
	ErrAllProxiesExpired = errors.New("proxypool: all proxies expired")
	ErrNoAvailableProxy  = errors.New("proxypool: no available proxy")
	ErrPoolClosed        = errors.New("proxypool: pool is closed")
)

// ProxyType 代理类型
type ProxyType string

const (
	ProxyTypeSOCKS5 ProxyType = "socks5"
	ProxyTypeHTTP   ProxyType = "http"
	ProxyTypeHTTPS  ProxyType = "https"
)

// Proxy 代理信息
type Proxy struct {
	Type      ProxyType         // 代理类型：SOCKS5, HTTP, HTTPS
	Host      string            // IP或域名
	Port      int               // 端口
	Username  string            // 用户名（可选）
	Password  string            // 密码（可选）
	ExpiredAt time.Time         // 过期时间
	Region    string            // 地区（可选）
	ISP       string            // 运营商（可选）
	Metadata  map[string]string // 其他元数据
}

// URL 返回代理URL
func (p *Proxy) URL() string {
	if p.Username != "" {
		return fmt.Sprintf("%s://%s:%s@%s:%d",
			p.Type, p.Username, p.Password, p.Host, p.Port)
	}
	return fmt.Sprintf("%s://%s:%d", p.Type, p.Host, p.Port)
}

// Provider 代理提供商接口
// 调用层需要实现此接口来对接不同的代理商
type Provider interface {
	// Fetch 拉取指定数量的代理
	// 返回的Proxy切片必须包含ExpiredAt字段，池会自动管理过期
	Fetch(ctx context.Context, count int) ([]Proxy, error)

	// Name 提供商名称，用于日志和监控
	Name() string
}

// SelectStrategy 代理选择策略
type SelectStrategy int

const (
	RoundRobin       SelectStrategy = iota // 轮询（默认）
	LeastUsed                              // 最少使用
	Random                                 // 随机
	WeightedByHealth                       // 按健康度加权
)

// Config 代理池配置
type Config struct {
	// ========== 必需配置 ==========
	Provider Provider // 代理提供商（必需）

	// ========== 池大小管理 ==========
	TargetSize    int     // 最小保有数量，默认100；不是最大容量限制
	LowWatermark  float64 // 兼容字段，保留给调用方读取
	HighWatermark float64 // 兼容字段，保留给调用方读取

	// ========== 启动配置 ==========
	StartupBatchSize   int // 启动时每批大小，默认20
	StartupConcurrency int // 启动时并发批次数，默认5（TargetSize/StartupBatchSize）

	// ========== 运行时刷新 ==========
	RefreshWindow  time.Duration        // 提前刷新窗口（代理过期前多久开始补充），默认15分钟
	RefreshBatch   int                  // 运行时每批补充数量，默认20
	RefreshAllowed func(time.Time) bool // 自动刷新时间窗控制，返回false时跳过后台自动补充

	// ========== 选择策略 ==========
	SelectStrategy SelectStrategy // 代理选择策略，默认RoundRobin

	// ========== HTTP客户端配置（req/v3） ==========
	// 连接复用
	EnableKeepAlive     bool          // 是否启用Keep-Alive，默认true
	MaxIdleConns        int           // 全局最大空闲连接数，默认100
	MaxIdleConnsPerHost int           // 每个host最大空闲连接数，默认10
	IdleConnTimeout     time.Duration // 空闲连接超时，默认90秒

	// 超时配置
	Timeout             time.Duration // 请求超时，默认5秒
	TLSHandshakeTimeout time.Duration // TLS握手超时，默认10秒

	// 其他req/v3配置
	DisableCompression bool // 禁用压缩，默认false
	MaxRedirects       int  // 最大重定向次数，默认10

	// ========== 健康检查（可选） ==========
	HealthCheck         bool          // 是否启用健康检查，默认false
	HealthCheckURL      string        // 健康检查URL，默认空
	HealthCheckInterval time.Duration // 健康检查间隔，默认5分钟
	MinHealthScore      float64       // 最低健康评分（0-1），低于此值剔除，默认0.3
	MaxConsecutiveFails int           // 最大连续失败次数，超过此值剔除，默认5
	MaxFailRate         float64       // 最大失败率（0-1），超过此值剔除，默认0.8

	// ========== 监控间隔 ==========
	MonitorInterval time.Duration // 水位线检查间隔，默认1分钟
	PruneInterval   time.Duration // 不健康代理剔除间隔，默认2分钟

	// ========== 日志配置 ==========
	Logger Logger // 自定义日志接口，默认nil（不输出）

	// ========== 预检配置 ==========
	PreCheck PreCheckConfig // 代理预检配置
}

// Logger 日志接口，允许用户自定义日志实现
type Logger interface {
	Printf(format string, v ...interface{})
	Errorf(format string, v ...interface{})
}

// Stats 池统计信息
type Stats struct {
	Total       int       // 总代理数
	Available   int       // 可用代理数（未过期+健康）
	Expired     int       // 过期代理数
	Unhealthy   int       // 不健康代理数
	Utilization float64   // 利用率（Available/TargetSize）
	LastRefresh time.Time // 最后刷新时间
	LastPrune   time.Time // 最后剔除时间

	// 使用统计
	TotalRequests int64 // 总请求次数
	TotalSuccess  int64 // 总成功次数
	TotalFailures int64 // 总失败次数

	// 每个代理的详细统计
	ProxyStats []ProxyStats
}

// ProxyStats 单个代理的统计信息
type ProxyStats struct {
	Proxy            Proxy
	UseCount         int64
	SuccessCount     int64
	FailCount        int64
	TimeoutCount     int64
	HealthScore      float64
	LastUsed         time.Time
	LastFailed       time.Time
	AvgLatency       time.Duration
	ConsecutiveFails int

	// 🆕 额外信息
	RealExitIP   string        // 真实出口IP（来自预检或首次使用）
	IsExpired    bool          // 是否已过期
	TimeToExpire time.Duration // 距离过期的时间
}

// applyDefaults 应用默认配置
func (c *Config) applyDefaults() {
	if c.TargetSize <= 0 {
		c.TargetSize = 100
	}
	if c.LowWatermark <= 0 {
		c.LowWatermark = 0.7
	}
	if c.HighWatermark <= 0 {
		c.HighWatermark = 0.9
	}
	if c.StartupBatchSize <= 0 {
		c.StartupBatchSize = 20
	}
	if c.StartupConcurrency <= 0 {
		c.StartupConcurrency = (c.TargetSize + c.StartupBatchSize - 1) / c.StartupBatchSize
		if c.StartupConcurrency > 10 {
			c.StartupConcurrency = 10 // 最多10个并发
		}
	}
	if c.RefreshWindow <= 0 {
		c.RefreshWindow = 15 * time.Minute
	}
	if c.RefreshBatch <= 0 {
		c.RefreshBatch = 20
	}

	// HTTP客户端默认配置
	if c.MaxIdleConns <= 0 {
		c.MaxIdleConns = 100
	}
	if c.MaxIdleConnsPerHost <= 0 {
		c.MaxIdleConnsPerHost = 10
	}
	if c.IdleConnTimeout <= 0 {
		c.IdleConnTimeout = 90 * time.Second
	}
	if c.Timeout <= 0 {
		c.Timeout = 5 * time.Second
	}
	if c.TLSHandshakeTimeout <= 0 {
		c.TLSHandshakeTimeout = 10 * time.Second
	}
	if c.MaxRedirects == 0 {
		c.MaxRedirects = 10
	}

	// 健康检查默认配置
	if c.HealthCheckInterval <= 0 {
		c.HealthCheckInterval = 5 * time.Minute
	}
	if c.MinHealthScore <= 0 {
		c.MinHealthScore = 0.3
	}
	if c.MaxConsecutiveFails <= 0 {
		c.MaxConsecutiveFails = 5
	}
	if c.MaxFailRate <= 0 {
		c.MaxFailRate = 0.8
	}

	// 监控间隔默认配置
	if c.MonitorInterval <= 0 {
		c.MonitorInterval = 1 * time.Minute
	}
	if c.PruneInterval <= 0 {
		c.PruneInterval = 2 * time.Minute
	}

	c.PreCheck.applyDefaults()
}

// validate 验证配置
func (c *Config) validate() error {
	if c.Provider == nil {
		return ErrNoProvider
	}
	if c.LowWatermark >= c.HighWatermark {
		return errors.New("proxypool: LowWatermark must be less than HighWatermark")
	}
	return nil
}
