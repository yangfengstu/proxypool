package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/yangfengstu/proxypool"
)

// IPZanProvider IP赞代理提供商
type IPZanProvider struct {
	baseURL      string // 提取链接（必需）
	defaultCount int    // 默认拉取数量
}

// IPZanConfig IP赞配置
type IPZanConfig struct {
	// 方式1：直接传提取链接（推荐）
	ExtractURL string // 完整的提取链接，如：https://service.ipzan.com/core-extract?num=1&no=xxx&minute=3&format=json&protocol=3&pool=quality&mode=auth&secret=xxx

	// 方式2：分别配置参数（兼容旧方式）
	No       string // 订单号
	Secret   string // 密钥
	Minute   int    // 代理有效期（分钟），默认3
	Protocol int    // 协议类型：1=HTTP/HTTPS, 3=SOCKS5，默认3
	Pool     string // 代理池类型，默认quality
	Mode     string // 认证模式，默认auth

	// 通用配置
	DefaultCount int // 默认拉取数量，0则使用链接中的num参数
}

// ipzanResponse IP赞API响应结构
type ipzanResponse struct {
	Data struct {
		List []struct {
			IP       string `json:"ip"`
			Port     string `json:"port"`
			Expired  int64  `json:"expired"`  // 过期时间戳（毫秒）
			Net      string `json:"net"`      // 网络类型（电信、联通等）
			Account  string `json:"account"`  // 账号
			Password string `json:"password"` // 密码
		} `json:"list"`
	} `json:"data"`
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}

// NewIPZanProvider 创建IP赞代理提供商
func NewIPZanProvider(cfg IPZanConfig) (*IPZanProvider, error) {
	provider := &IPZanProvider{
		defaultCount: cfg.DefaultCount,
	}

	// 方式1：使用提取链接（推荐）
	if cfg.ExtractURL != "" {
		// 验证URL格式
		if _, err := url.Parse(cfg.ExtractURL); err != nil {
			return nil, fmt.Errorf("ipzan: invalid extract URL: %w", err)
		}

		// 确保包含必要参数
		if !strings.Contains(cfg.ExtractURL, "format=json") {
			return nil, fmt.Errorf("ipzan: extract URL must contain 'format=json'")
		}

		provider.baseURL = cfg.ExtractURL
		return provider, nil
	}

	// 方式2：使用参数构造URL（兼容旧方式）
	if cfg.No == "" || cfg.Secret == "" {
		return nil, fmt.Errorf("ipzan: either ExtractURL or (No + Secret) is required")
	}

	// 设置默认值
	if cfg.Minute <= 0 {
		cfg.Minute = 3
	}
	if cfg.Protocol == 0 {
		cfg.Protocol = 3 // 默认SOCKS5
	}
	if cfg.Pool == "" {
		cfg.Pool = "quality"
	}
	if cfg.Mode == "" {
		cfg.Mode = "auth"
	}

	// 构造URL
	provider.baseURL = fmt.Sprintf("https://service.ipzan.com/core-extract?num=1&no=%s&minute=%d&format=json&protocol=%d&pool=%s&mode=%s&secret=%s",
		cfg.No, cfg.Minute, cfg.Protocol, cfg.Pool, cfg.Mode, cfg.Secret)

	return provider, nil
}

// Fetch 实现Provider接口，拉取指定数量的代理
func (p *IPZanProvider) Fetch(ctx context.Context, count int) ([]proxypool.Proxy, error) {
	// 使用默认数量
	if count <= 0 && p.defaultCount > 0 {
		count = p.defaultCount
	}
	if count <= 0 {
		count = 1
	}

	// IP赞单次最多拉取100个
	if count > 100 {
		count = 100
	}

	// 构造请求URL（替换或添加num参数）
	reqURL := p.baseURL
	parsedURL, _ := url.Parse(reqURL)
	query := parsedURL.Query()
	query.Set("num", strconv.Itoa(count))
	parsedURL.RawQuery = query.Encode()
	reqURL = parsedURL.String()

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("ipzan: create request failed: %w", err)
	}

	// 发起请求
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ipzan: request failed: %w", err)
	}
	defer resp.Body.Close()

	// 检查HTTP状态码
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("ipzan: unexpected status code: %d", resp.StatusCode)
	}

	// 解析响应
	var result ipzanResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ipzan: decode response failed: %w", err)
	}

	// 检查业务状态码
	if result.Code != 0 {
		return nil, fmt.Errorf("ipzan: api error (code=%d): %s", result.Code, result.Message)
	}

	if len(result.Data.List) == 0 {
		return nil, fmt.Errorf("ipzan: no proxies returned")
	}

	// 推断代理类型（从URL中的protocol参数）
	proxyType := proxypool.ProxyTypeSOCKS5
	parsedURL, _ = url.Parse(p.baseURL)
	if protocol := parsedURL.Query().Get("protocol"); protocol == "1" {
		proxyType = proxypool.ProxyTypeHTTP
	}

	// 转换为标准Proxy结构
	proxies := make([]proxypool.Proxy, 0, len(result.Data.List))
	for _, item := range result.Data.List {
		// 解析端口
		port, ok := parseIPZanPort(item.Port)
		if !ok {
			continue
		}

		// 转换过期时间（毫秒 -> time.Time）
		expiredAt := time.UnixMilli(item.Expired)

		proxy := proxypool.Proxy{
			Type:      proxyType,
			Host:      item.IP,
			Port:      port,
			Username:  item.Account,
			Password:  item.Password,
			ExpiredAt: expiredAt,
			ISP:       item.Net,
			Metadata: map[string]string{
				"provider": "ipzan",
				"net":      item.Net,
			},
		}

		proxies = append(proxies, proxy)
	}

	if len(proxies) == 0 {
		return nil, fmt.Errorf("ipzan: no valid proxies returned")
	}

	return proxies, nil
}

func parseIPZanPort(value string) (int, bool) {
	port, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || port <= 0 || port > 65535 {
		return 0, false
	}
	return port, true
}

// Name 实现Provider接口
func (p *IPZanProvider) Name() string {
	return "ipzan"
}
