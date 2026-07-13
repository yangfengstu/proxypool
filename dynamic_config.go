package proxypool

import (
	"sync/atomic"
	"time"
)

// DynamicConfig 动态配置（可在运行时变更）
type DynamicConfig struct {
	// 池大小
	targetSize    atomic.Int32
	lowWatermark  atomic.Value // float64
	highWatermark atomic.Value // float64

	// 监控开关
	autoRefresh atomic.Bool
	autoPrune   atomic.Bool

	// 间隔时间
	monitorInterval atomic.Value // time.Duration
	pruneInterval   atomic.Value // time.Duration

	// 健康阈值
	minHealthScore      atomic.Value // float64
	maxConsecutiveFails atomic.Int32
	maxFailRate         atomic.Value // float64

	// 预检配置
	precheckEnabled     atomic.Bool
	precheckMaxLatency  atomic.Value // time.Duration
	precheckConcurrency atomic.Int32
}

// newDynamicConfig 创建动态配置
func newDynamicConfig(cfg Config) *DynamicConfig {
	dc := &DynamicConfig{}

	// 初始化
	dc.targetSize.Store(int32(cfg.TargetSize))
	dc.lowWatermark.Store(cfg.LowWatermark)
	dc.highWatermark.Store(cfg.HighWatermark)
	dc.autoRefresh.Store(true)
	dc.autoPrune.Store(true)
	dc.monitorInterval.Store(cfg.MonitorInterval)
	dc.pruneInterval.Store(cfg.PruneInterval)
	dc.minHealthScore.Store(cfg.MinHealthScore)
	dc.maxConsecutiveFails.Store(int32(cfg.MaxConsecutiveFails))
	dc.maxFailRate.Store(cfg.MaxFailRate)
	dc.precheckEnabled.Store(cfg.PreCheck.Enabled)
	dc.precheckMaxLatency.Store(cfg.PreCheck.MaxLatency)
	dc.precheckConcurrency.Store(int32(cfg.PreCheck.Concurrency))

	return dc
}

// UpdateConfig 更新配置选项
type UpdateConfig struct {
	// 池大小
	TargetSize    *int     // 目标池大小
	LowWatermark  *float64 // 低水位
	HighWatermark *float64 // 高水位

	// 监控开关
	AutoRefresh *bool // 是否自动刷新
	AutoPrune   *bool // 是否自动剔除

	// 间隔时间
	MonitorInterval *time.Duration // 监控间隔
	PruneInterval   *time.Duration // 剔除间隔

	// 健康阈值
	MinHealthScore      *float64 // 最低健康评分
	MaxConsecutiveFails *int     // 最大连续失败
	MaxFailRate         *float64 // 最大失败率

	// 预检配置
	PreCheckEnabled     *bool          // 是否启用预检
	PreCheckMaxLatency  *time.Duration // 预检最大延迟
	PreCheckConcurrency *int           // 预检并发数
}

// UpdateConfig 实时更新配置
func (p *Pool) UpdateConfig(update UpdateConfig) {
	// 池大小
	if update.TargetSize != nil {
		p.dynamicConfig.targetSize.Store(int32(*update.TargetSize))
		p.logf("Config updated: TargetSize = %d", *update.TargetSize)
	}
	if update.LowWatermark != nil {
		p.dynamicConfig.lowWatermark.Store(*update.LowWatermark)
		p.logf("Config updated: LowWatermark = %.2f", *update.LowWatermark)
	}
	if update.HighWatermark != nil {
		p.dynamicConfig.highWatermark.Store(*update.HighWatermark)
		p.logf("Config updated: HighWatermark = %.2f", *update.HighWatermark)
	}

	// 监控开关
	if update.AutoRefresh != nil {
		p.dynamicConfig.autoRefresh.Store(*update.AutoRefresh)
		p.logf("Config updated: AutoRefresh = %v", *update.AutoRefresh)
	}
	if update.AutoPrune != nil {
		p.dynamicConfig.autoPrune.Store(*update.AutoPrune)
		p.logf("Config updated: AutoPrune = %v", *update.AutoPrune)
	}

	// 间隔时间（需要重启后台任务）
	if update.MonitorInterval != nil {
		old := p.dynamicConfig.monitorInterval.Load().(time.Duration)
		p.dynamicConfig.monitorInterval.Store(*update.MonitorInterval)
		if old != *update.MonitorInterval {
			p.logf("Config updated: MonitorInterval = %v (will take effect on next check)", *update.MonitorInterval)
		}
	}
	if update.PruneInterval != nil {
		old := p.dynamicConfig.pruneInterval.Load().(time.Duration)
		p.dynamicConfig.pruneInterval.Store(*update.PruneInterval)
		if old != *update.PruneInterval {
			p.logf("Config updated: PruneInterval = %v (will take effect on next check)", *update.PruneInterval)
		}
	}

	// 健康阈值
	if update.MinHealthScore != nil {
		p.dynamicConfig.minHealthScore.Store(*update.MinHealthScore)
		p.logf("Config updated: MinHealthScore = %.2f", *update.MinHealthScore)
	}
	if update.MaxConsecutiveFails != nil {
		p.dynamicConfig.maxConsecutiveFails.Store(int32(*update.MaxConsecutiveFails))
		p.logf("Config updated: MaxConsecutiveFails = %d", *update.MaxConsecutiveFails)
	}
	if update.MaxFailRate != nil {
		p.dynamicConfig.maxFailRate.Store(*update.MaxFailRate)
		p.logf("Config updated: MaxFailRate = %.2f", *update.MaxFailRate)
	}

	// 预检配置
	if update.PreCheckEnabled != nil {
		p.dynamicConfig.precheckEnabled.Store(*update.PreCheckEnabled)
		p.logf("Config updated: PreCheckEnabled = %v", *update.PreCheckEnabled)
	}
	if update.PreCheckMaxLatency != nil {
		p.dynamicConfig.precheckMaxLatency.Store(*update.PreCheckMaxLatency)
		p.logf("Config updated: PreCheckMaxLatency = %v", *update.PreCheckMaxLatency)
	}
	if update.PreCheckConcurrency != nil {
		p.dynamicConfig.precheckConcurrency.Store(int32(*update.PreCheckConcurrency))
		p.logf("Config updated: PreCheckConcurrency = %d", *update.PreCheckConcurrency)
	}
}

// GetCurrentConfig 获取当前配置
func (p *Pool) GetCurrentConfig() CurrentConfig {
	return CurrentConfig{
		TargetSize:          int(p.dynamicConfig.targetSize.Load()),
		LowWatermark:        p.dynamicConfig.lowWatermark.Load().(float64),
		HighWatermark:       p.dynamicConfig.highWatermark.Load().(float64),
		AutoRefresh:         p.dynamicConfig.autoRefresh.Load(),
		AutoPrune:           p.dynamicConfig.autoPrune.Load(),
		MonitorInterval:     p.dynamicConfig.monitorInterval.Load().(time.Duration),
		PruneInterval:       p.dynamicConfig.pruneInterval.Load().(time.Duration),
		MinHealthScore:      p.dynamicConfig.minHealthScore.Load().(float64),
		MaxConsecutiveFails: int(p.dynamicConfig.maxConsecutiveFails.Load()),
		MaxFailRate:         p.dynamicConfig.maxFailRate.Load().(float64),
		PreCheckEnabled:     p.dynamicConfig.precheckEnabled.Load(),
		PreCheckMaxLatency:  p.dynamicConfig.precheckMaxLatency.Load().(time.Duration),
		PreCheckConcurrency: int(p.dynamicConfig.precheckConcurrency.Load()),
	}
}

// CurrentConfig 当前配置快照
type CurrentConfig struct {
	TargetSize          int
	LowWatermark        float64
	HighWatermark       float64
	AutoRefresh         bool
	AutoPrune           bool
	MonitorInterval     time.Duration
	PruneInterval       time.Duration
	MinHealthScore      float64
	MaxConsecutiveFails int
	MaxFailRate         float64
	PreCheckEnabled     bool
	PreCheckMaxLatency  time.Duration
	PreCheckConcurrency int
}

// 便捷方法：获取动态配置值
func (p *Pool) getTargetSize() int {
	return int(p.dynamicConfig.targetSize.Load())
}

func (p *Pool) isAutoRefreshEnabled() bool {
	return p.dynamicConfig.autoRefresh.Load()
}

func (p *Pool) isAutoPruneEnabled() bool {
	return p.dynamicConfig.autoPrune.Load()
}

func (p *Pool) getMonitorInterval() time.Duration {
	return p.dynamicConfig.monitorInterval.Load().(time.Duration)
}

func (p *Pool) getPruneInterval() time.Duration {
	return p.dynamicConfig.pruneInterval.Load().(time.Duration)
}

func (p *Pool) getMinHealthScore() float64 {
	return p.dynamicConfig.minHealthScore.Load().(float64)
}

func (p *Pool) getMaxConsecutiveFails() int {
	return int(p.dynamicConfig.maxConsecutiveFails.Load())
}

func (p *Pool) getMaxFailRate() float64 {
	return p.dynamicConfig.maxFailRate.Load().(float64)
}

func (p *Pool) isPreCheckEnabled() bool {
	return p.dynamicConfig.precheckEnabled.Load()
}

func (p *Pool) getPreCheckMaxLatency() time.Duration {
	return p.dynamicConfig.precheckMaxLatency.Load().(time.Duration)
}

func (p *Pool) getPreCheckConcurrency() int {
	return int(p.dynamicConfig.precheckConcurrency.Load())
}
