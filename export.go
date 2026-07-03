package proxypool

import (
	"encoding/json"
	"fmt"
	"time"
)

// ProxyDetail 代理详细信息（用于导出）
type ProxyDetail struct {
	// 基础信息
	ProxyURL    string `json:"proxy_url"`     // 代理URL（含认证）
	ProxyHost   string `json:"proxy_host"`    // 代理地址
	ProxyPort   int    `json:"proxy_port"`    // 代理端口
	ProxyType   string `json:"proxy_type"`    // 代理类型
	Username    string `json:"username"`      // 用户名
	Region      string `json:"region"`        // 地区
	ISP         string `json:"isp"`           // 运营商

	// 🆕 出口IP信息
	RealExitIP  string `json:"real_exit_ip"`  // 真实出口IP
	ExitIPFrom  string `json:"exit_ip_from"`  // IP来源（precheck/first_use/unknown）

	// 生命周期
	ExpiredAt    time.Time `json:"expired_at"`     // 过期时间
	IsExpired    bool      `json:"is_expired"`     // 是否已过期
	TimeToExpire string    `json:"time_to_expire"` // 距离过期（人类可读）

	// 使用统计
	UseCount     int64   `json:"use_count"`      // 使用次数
	SuccessCount int64   `json:"success_count"`  // 成功次数
	FailCount    int64   `json:"fail_count"`     // 失败次数
	TimeoutCount int64   `json:"timeout_count"`  // 超时次数
	SuccessRate  float64 `json:"success_rate"`   // 成功率

	// 健康状态
	HealthScore      float64   `json:"health_score"`       // 健康评分
	ConsecutiveFails int       `json:"consecutive_fails"`  // 连续失败次数
	AvgLatency       string    `json:"avg_latency"`        // 平均延迟（人类可读）
	LastUsed         time.Time `json:"last_used"`          // 最后使用时间
	LastFailed       time.Time `json:"last_failed"`        // 最后失败时间

	// 预检信息
	PreCheckLatency string    `json:"precheck_latency,omitempty"` // 预检延迟
	PreCheckTime    time.Time `json:"precheck_time,omitempty"`    // 预检时间
}

// GetAllProxyDetails 获取所有代理的详细信息
func (p *Pool) GetAllProxyDetails() []ProxyDetail {
	p.mu.RLock()
	defer p.mu.RUnlock()

	now := time.Now()
	details := make([]ProxyDetail, 0, len(p.proxies))

	for _, pc := range p.proxies {
		pc.mu.Lock()

		// 计算成功率
		var successRate float64
		if pc.useCount > 0 {
			successRate = float64(pc.successCount) / float64(pc.useCount) * 100
		}

		// 计算平均延迟
		var avgLatency time.Duration
		if pc.useCount > 0 {
			avgLatency = pc.totalLatency / time.Duration(pc.useCount)
		}

		// 获取真实出口IP
		realExitIP := pc.Proxy.Metadata["precheck_real_ip"]
		exitIPFrom := "unknown"
		if realExitIP != "" {
			exitIPFrom = "precheck"
		} else if realExitIP = pc.Proxy.Metadata["first_use_exit_ip"]; realExitIP != "" {
			exitIPFrom = "first_use"
		}

		// 过期信息
		isExpired := now.After(pc.ExpireAt)
		timeToExpire := pc.ExpireAt.Sub(now)

		detail := ProxyDetail{
			// 基础信息
			ProxyURL:  pc.Proxy.URL(),
			ProxyHost: pc.Proxy.Host,
			ProxyPort: pc.Proxy.Port,
			ProxyType: string(pc.Proxy.Type),
			Username:  pc.Proxy.Username,
			Region:    pc.Proxy.Region,
			ISP:       pc.Proxy.ISP,

			// 出口IP
			RealExitIP: realExitIP,
			ExitIPFrom: exitIPFrom,

			// 生命周期
			ExpiredAt:    pc.ExpireAt,
			IsExpired:    isExpired,
			TimeToExpire: formatDuration(timeToExpire),

			// 使用统计
			UseCount:     pc.useCount,
			SuccessCount: pc.successCount,
			FailCount:    pc.failCount,
			TimeoutCount: pc.timeoutCount,
			SuccessRate:  successRate,

			// 健康状态
			HealthScore:      pc.healthScore,
			ConsecutiveFails: pc.consecutiveFails,
			AvgLatency:       formatDuration(avgLatency),
			LastUsed:         pc.lastUsed,
			LastFailed:       pc.lastFailed,

			// 预检信息
			PreCheckLatency: pc.Proxy.Metadata["precheck_latency"],
		}

		if precheckAt := pc.Proxy.Metadata["precheck_at"]; precheckAt != "" {
			if t, err := time.Parse(time.RFC3339, precheckAt); err == nil {
				detail.PreCheckTime = t
			}
		}

		pc.mu.Unlock()
		details = append(details, detail)
	}

	return details
}

// ExportProxyDetailsJSON 导出代理详情为JSON
func (p *Pool) ExportProxyDetailsJSON() (string, error) {
	details := p.GetAllProxyDetails()
	data, err := json.MarshalIndent(details, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GetProxyByHost 根据代理地址查找代理详情
func (p *Pool) GetProxyByHost(host string, port int) *ProxyDetail {
	details := p.GetAllProxyDetails()
	for _, detail := range details {
		if detail.ProxyHost == host && detail.ProxyPort == port {
			return &detail
		}
	}
	return nil
}

// formatDuration 格式化时间间隔为人类可读
func formatDuration(d time.Duration) string {
	if d < 0 {
		return "expired"
	}

	if d < time.Second {
		return d.Round(time.Millisecond).String()
	}
	if d < time.Minute {
		return d.Round(time.Second).String()
	}
	if d < time.Hour {
		return d.Round(time.Second).String()
	}
	if d < 24*time.Hour {
		return d.Round(time.Minute).String()
	}
	return d.Round(time.Hour).String()
}

// ProxyInfo 代理信息（用于日志）
type ProxyInfo struct {
	Host     string
	Port     int
	Type     string
	Region   string
	ISP      string
	ExitIP   string
	UseCount int64
	Health   float64
}

// GetProxyInfo 获取代理信息（用于日志打印）
func (p *Pool) GetProxyInfo(proxy *Proxy) ProxyInfo {
	if proxy == nil {
		return ProxyInfo{}
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, pc := range p.proxies {
		if pc.Proxy.Host == proxy.Host && pc.Proxy.Port == proxy.Port {
			pc.mu.Lock()
			info := ProxyInfo{
				Host:     pc.Proxy.Host,
				Port:     pc.Proxy.Port,
				Type:     string(pc.Proxy.Type),
				Region:   pc.Proxy.Region,
				ISP:      pc.Proxy.ISP,
				ExitIP:   pc.Proxy.Metadata["precheck_real_ip"],
				UseCount: pc.useCount,
				Health:   pc.healthScore,
			}
			if info.ExitIP == "" {
				info.ExitIP = pc.Proxy.Metadata["first_use_exit_ip"]
			}
			pc.mu.Unlock()
			return info
		}
	}

	return ProxyInfo{
		Host: proxy.Host,
		Port: proxy.Port,
		Type: string(proxy.Type),
	}
}

// String 代理信息的字符串表示（用于日志）
func (pi ProxyInfo) String() string {
	if pi.ExitIP != "" {
		return formatProxyWithIP(pi.Host, pi.Port, pi.Type, pi.ExitIP, pi.UseCount, pi.Health)
	}
	return formatProxyBasic(pi.Host, pi.Port, pi.Type, pi.UseCount, pi.Health)
}

func formatProxyWithIP(host string, port int, proxyType, exitIP string, useCount int64, health float64) string {
	return formatString(
		"proxy=%s:%d type=%s exit_ip=%s use=%d health=%.2f",
		host, port, proxyType, exitIP, useCount, health,
	)
}

func formatProxyBasic(host string, port int, proxyType string, useCount int64, health float64) string {
	return formatString(
		"proxy=%s:%d type=%s use=%d health=%.2f",
		host, port, proxyType, useCount, health,
	)
}

func formatString(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}

// 在Proxy类型上添加便捷方法
func (p *Proxy) MaskPassword() string {
	if len(p.Password) <= 4 {
		return "****"
	}
	return p.Password[:2] + "****" + p.Password[len(p.Password)-2:]
}

func (p *Proxy) SafeURL() string {
	if p.Username != "" {
		return formatString("%s://%s:%s@%s:%d",
			p.Type, p.Username, p.MaskPassword(), p.Host, p.Port)
	}
	return formatString("%s://%s:%d", p.Type, p.Host, p.Port)
}
