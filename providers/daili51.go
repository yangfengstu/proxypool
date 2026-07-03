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

// Daili51Provider 51代理提供商
type Daili51Provider struct {
	baseURL      string // 提取链接（必需）
	username     string // 代理认证用户名（从提取链接解析）
	password     string // 代理认证密码（从提取链接解析）
	defaultCount int    // 默认拉取数量
}

// Daili51Config 51代理配置
type Daili51Config struct {
	// 方式1：直接传提取链接（推荐）
	ExtractURL string // 完整的提取链接

	// 通用配置
	DefaultCount int // 默认拉取数量，0则使用链接中的qty参数
}

// daili51Response 51代理API响应结构
type daili51Response struct {
	Msg     string `json:"msg"`
	Code    int    `json:"code"`
	Success string `json:"success"`
	Data    []struct {
		ExpireTimeMillis string `json:"expireTimeMillis"` // 过期时间戳（毫秒，字符串格式）
		IPAddress        string `json:"ipaddress"`        // 地区编码
		ExpireTime       string `json:"expireTime"`       // 过期时间（格式：2026-07-03 14:26:26）
		IP               string `json:"ip"`               // IP:端口
		ISP              string `json:"isp"`              // 运营商（移动、联通、电信）
		IPAddressName    string `json:"IpAddressName"`    // 地区名称
	} `json:"data"`
}

// NewDaili51Provider 创建51代理提供商
func NewDaili51Provider(cfg Daili51Config) (*Daili51Provider, error) {
	if cfg.ExtractURL == "" {
		return nil, fmt.Errorf("daili51: ExtractURL is required")
	}

	// 验证URL格式
	parsedURL, err := url.Parse(cfg.ExtractURL)
	if err != nil {
		return nil, fmt.Errorf("daili51: invalid extract URL: %w", err)
	}

	// 确保包含必要参数
	if !strings.Contains(cfg.ExtractURL, "format=json") {
		return nil, fmt.Errorf("daili51: extract URL must contain 'format=json'")
	}

	// 从URL中提取认证信息
	query := parsedURL.Query()
	username := query.Get("accessName")
	password := query.Get("accessPassword")

	if username == "" || password == "" {
		return nil, fmt.Errorf("daili51: extract URL must contain accessName and accessPassword")
	}

	provider := &Daili51Provider{
		baseURL:      cfg.ExtractURL,
		username:     username,
		password:     password,
		defaultCount: cfg.DefaultCount,
	}

	return provider, nil
}

// Fetch 实现Provider接口，拉取指定数量的代理
func (p *Daili51Provider) Fetch(ctx context.Context, count int) ([]proxypool.Proxy, error) {
	// 使用默认数量
	if count <= 0 && p.defaultCount > 0 {
		count = p.defaultCount
	}
	if count <= 0 {
		count = 1
	}

	// 51代理单次最多拉取200个
	if count > 200 {
		count = 200
	}

	// 构造请求URL（替换或添加qty参数）
	reqURL := p.baseURL
	parsedURL, _ := url.Parse(reqURL)
	query := parsedURL.Query()
	query.Set("qty", strconv.Itoa(count))
	parsedURL.RawQuery = query.Encode()
	reqURL = parsedURL.String()

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("daili51: create request failed: %w", err)
	}

	// 发起请求
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("daili51: request failed: %w", err)
	}
	defer resp.Body.Close()

	// 检查HTTP状态码
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("daili51: unexpected status code: %d", resp.StatusCode)
	}

	// 解析响应
	var result daili51Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("daili51: decode response failed: %w", err)
	}

	// 检查业务状态码
	if result.Code != 0 || result.Success != "true" {
		return nil, fmt.Errorf("daili51: api error (code=%d): %s", result.Code, result.Msg)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("daili51: no proxies returned")
	}

	// 推断代理类型（从URL中的port参数）
	// port=1: HTTP, port=2: SOCKS5
	proxyType := proxypool.ProxyTypeSOCKS5
	parsedURL, _ = url.Parse(p.baseURL)
	if port := parsedURL.Query().Get("port"); port == "1" {
		proxyType = proxypool.ProxyTypeHTTP
	}

	// 转换为标准Proxy结构
	proxies := make([]proxypool.Proxy, 0, len(result.Data))
	for _, item := range result.Data {
		// 解析IP和端口（格式：221.229.220.22:38792）
		parts := strings.Split(item.IP, ":")
		if len(parts) != 2 {
			continue // 跳过格式错误的
		}
		host := parts[0]
		port, _ := strconv.Atoi(parts[1])

		// 解析过期时间（毫秒时间戳，字符串格式）
		expiredMillis, _ := strconv.ParseInt(item.ExpireTimeMillis, 10, 64)
		expiredAt := time.UnixMilli(expiredMillis)

		proxy := proxypool.Proxy{
			Type:      proxyType,
			Host:      host,
			Port:      port,
			Username:  p.username, // 所有代理使用相同的认证信息
			Password:  p.password,
			ExpiredAt: expiredAt,
			Region:    item.IPAddressName,
			ISP:       item.ISP,
			Metadata: map[string]string{
				"provider":    "daili51",
				"region_code": item.IPAddress,
				"region_name": item.IPAddressName,
				"isp":         item.ISP,
			},
		}

		proxies = append(proxies, proxy)
	}

	return proxies, nil
}

// Name 实现Provider接口
func (p *Daili51Provider) Name() string {
	return "daili51"
}
