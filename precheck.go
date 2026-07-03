package proxypool

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/imroc/req/v3"
)

// PreCheckConfig 预检配置
type PreCheckConfig struct {
	Enabled         bool          // 是否启用预检，默认false
	Timeout         time.Duration // 预检超时，默认5秒
	MaxLatency      time.Duration // 最大允许延迟，超过此值剔除，默认3秒
	Concurrency     int           // 并发数，默认10
	CheckURLs       []string      // 检测URL列表，默认使用内置列表
	RequireRealIP   bool          // 是否要求获取真实出口IP，默认true
	MinSuccessCount int           // 最少成功次数（针对多个CheckURL），默认1
}

// PreCheckResult 预检结果
type PreCheckResult struct {
	Proxy     Proxy
	Success   bool
	Latency   time.Duration
	RealIP    string
	Error     error
	CheckedAt time.Time
}

// defaultCheckURLs 默认检测URL列表
var defaultCheckURLs = []string{
	"https://api.ipify.org?format=json", // 主要
	"https://ifconfig.me/ip",            // 备用1
	"https://api.myip.com",              // 备用2
	"https://checkip.amazonaws.com",     // 备用3 (返回纯文本)
	"https://icanhazip.com",             // 备用4 (返回纯文本)
}

// applyPreCheckDefaults 应用预检默认配置
func (c *PreCheckConfig) applyDefaults() {
	if c.Timeout <= 0 {
		c.Timeout = 5 * time.Second
	}
	if c.MaxLatency <= 0 {
		c.MaxLatency = 3 * time.Second
	}
	if c.Concurrency <= 0 {
		c.Concurrency = 10
	}
	if len(c.CheckURLs) == 0 {
		c.CheckURLs = defaultCheckURLs
	}
	if c.MinSuccessCount <= 0 {
		c.MinSuccessCount = 1
	}
}

// preCheckProxies 预检代理列表
func (p *Pool) preCheckProxies(proxies []Proxy) []Proxy {
	if !p.isPreCheckEnabled() {
		return proxies
	}

	if len(proxies) == 0 {
		return proxies
	}

	p.logf("Pre-checking %d proxies with %d workers...", len(proxies), p.getPreCheckConcurrency())
	startTime := time.Now()

	// 创建工作池
	jobs := make(chan Proxy, len(proxies))
	results := make(chan PreCheckResult, len(proxies))

	// 启动workers
	var wg sync.WaitGroup
	for i := 0; i < p.getPreCheckConcurrency(); i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for proxy := range jobs {
				result := p.checkProxy(proxy, workerID)
				results <- result
			}
		}(i)
	}

	// 发送任务
	for _, proxy := range proxies {
		jobs <- proxy
	}
	close(jobs)

	// 等待所有worker完成
	go func() {
		wg.Wait()
		close(results)
	}()

	// 收集结果
	validProxies := make([]Proxy, 0, len(proxies))
	var stats struct {
		total   int
		success int
		timeout int
		slow    int
		noIP    int
		other   int
	}

	for result := range results {
		stats.total++

		if !result.Success {
			if result.Error != nil {
				errMsg := result.Error.Error()
				if errMsg == "timeout" || errMsg == "context deadline exceeded" {
					stats.timeout++
				} else {
					stats.other++
				}
			}
			p.logf("  ✗ %s:%d - Failed: %v", result.Proxy.Host, result.Proxy.Port, result.Error)
			continue
		}

		// 检查延迟
		if result.Latency > p.getPreCheckMaxLatency() {
			stats.slow++
			p.logf("  ✗ %s:%d - Too slow: %v (max: %v)",
				result.Proxy.Host, result.Proxy.Port, result.Latency, p.getPreCheckMaxLatency())
			continue
		}

		// 检查是否获取到真实IP
		if p.config.PreCheck.RequireRealIP && result.RealIP == "" {
			stats.noIP++
			p.logf("  ✗ %s:%d - No real IP detected", result.Proxy.Host, result.Proxy.Port)
			continue
		}

		stats.success++
		p.logf("  ✓ %s:%d - OK (latency: %v, IP: %s)",
			result.Proxy.Host, result.Proxy.Port, result.Latency, result.RealIP)

		// 保存检测结果到Metadata
		proxy := result.Proxy
		if proxy.Metadata == nil {
			proxy.Metadata = make(map[string]string)
		}
		proxy.Metadata["precheck_latency"] = result.Latency.String()
		proxy.Metadata["precheck_real_ip"] = result.RealIP
		proxy.Metadata["precheck_at"] = result.CheckedAt.Format(time.RFC3339)

		validProxies = append(validProxies, proxy)
	}

	elapsed := time.Since(startTime)
	p.logf("Pre-check completed in %v: %d/%d passed (timeout: %d, slow: %d, no_ip: %d, other: %d)",
		elapsed, stats.success, stats.total, stats.timeout, stats.slow, stats.noIP, stats.other)

	return validProxies
}

// checkProxy 检查单个代理
func (p *Pool) checkProxy(proxy Proxy, workerID int) PreCheckResult {
	result := PreCheckResult{
		Proxy:     proxy,
		Success:   false,
		CheckedAt: time.Now(),
	}

	// 创建临时客户端（用于预检）
	client := req.C().
		SetProxyURL(proxy.URL()).
		SetTimeout(p.config.PreCheck.Timeout).
		SetCommonRetryCount(0).
		DisableKeepAlives() // 预检不需要Keep-Alive

	// 尝试多个检测URL
	successCount := 0
	var firstLatency time.Duration
	var firstIP string

	for _, checkURL := range p.config.PreCheck.CheckURLs {
		ctx, cancel := context.WithTimeout(context.Background(), p.config.PreCheck.Timeout)

		start := time.Now()
		resp, err := client.R().SetContext(ctx).Get(checkURL)
		latency := time.Since(start)
		cancel()

		if err != nil {
			result.Error = err
			continue
		}

		if resp.StatusCode != 200 {
			result.Error = fmt.Errorf("status code: %d", resp.StatusCode)
			continue
		}

		// 提取IP地址
		ip := extractIPFromResponse(checkURL, resp.String())

		// 记录第一次成功的结果
		if successCount == 0 {
			firstLatency = latency
			firstIP = ip
		}

		successCount++

		// 达到最小成功次数即可
		if successCount >= p.config.PreCheck.MinSuccessCount {
			break
		}
	}

	// 判断是否成功
	if successCount >= p.config.PreCheck.MinSuccessCount {
		result.Success = true
		result.Latency = firstLatency
		result.RealIP = firstIP
		result.Error = nil
	} else if result.Error == nil {
		result.Error = fmt.Errorf("insufficient successful checks: %d/%d", successCount, p.config.PreCheck.MinSuccessCount)
	}

	return result
}

// extractIPFromResponse 从响应中提取IP地址
func extractIPFromResponse(url, body string) string {
	// 尝试JSON解析（ipify, myip等）
	var jsonResult struct {
		IP string `json:"ip"`
	}
	if err := json.Unmarshal([]byte(body), &jsonResult); err == nil && jsonResult.IP != "" {
		return jsonResult.IP
	}

	// 尝试纯文本IP（checkip.amazonaws.com, icanhazip.com等）
	// 去除空白字符
	body = trimSpace(body)

	// 简单验证是否是IP格式（IPv4或IPv6）
	if isValidIP(body) {
		return body
	}

	return ""
}

// trimSpace 去除字符串前后的空白字符
func trimSpace(s string) string {
	start := 0
	end := len(s)

	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}

	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}

	return s[start:end]
}

// isValidIP 验证IP格式
func isValidIP(s string) bool {
	return net.ParseIP(s) != nil
}
