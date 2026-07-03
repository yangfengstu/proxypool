# Live Test Guide

## 🧪 如何运行Live测试

### IP赞测试

```bash
cd /Users/leo/Workspace/apps/proxypool

go run cmd/live_test/main.go \
  -provider=ipzan \
  -url='https://service.ipzan.com/core-extract?num=1&no=YOUR_NO&minute=3&format=json&protocol=3&pool=quality&mode=auth&secret=YOUR_SECRET' \
  -count=5
```

### 51代理测试

```bash
go run cmd/live_test/main.go \
  -provider=daili51 \
  -url='http://capi.51daili.com/traffic/getip?linePoolIndex=1&packid=12&time=11&qty=1&port=2&format=json&field=ipport,expiretime,regioncode,isptype&ct=1&rid=YOUR_RID&uid=YOUR_UID&accessName=YOUR_NAME&accessPassword=YOUR_PASSWORD' \
  -count=5
```

### 参数说明

- `-provider` : 代理商名称（ipzan 或 daili51）
- `-url` : 完整的提取链接
- `-count` : 拉取代理数量，默认5
- `-test` : 测试URL，默认 `https://api.ipify.org?format=json`

## 📋 测试内容

Live测试会执行以下4个测试：

### Test 1: 拉取代理
- ✅ 验证API连接
- ✅ 验证返回数据格式
- ✅ 显示代理详细信息

### Test 2: 创建代理池
- ✅ 验证Pool初始化
- ✅ 验证并发拉取

### Test 3: 连通性测试
- ✅ 使用代理发起真实HTTP请求
- ✅ 测试延迟
- ✅ 验证响应

### Test 4: 统计信息
- ✅ 查看池状态
- ✅ 成功率统计

## 🎯 预期输出

```
🧪 ProxyPool Live Test
==================================================
Provider: ipzan
Count: 5
Test URL: https://api.ipify.org?format=json

✅ Provider created: ipzan

📡 Test 1: Fetching proxies...
✅ Fetched 5 proxies

📋 Proxy Details:

[1] socks5://DH5LJ6MUMTO:u0******15o@218.95.39.77:11638
    Type: socks5
    Host: 218.95.39.77:11638
    Username: DH5LJ6MUMTO
    Password: u0******15o
    Expired: 2026-07-03 14:26:26 (3 minutes left)
    ISP: 电信

...

🏊 Test 2: Creating proxy pool...
✅ Pool created with 5 proxies

🌐 Test 3: Testing proxy connectivity...

[Test 1] Using proxy: 218.95.39.77
  ✅ Success! Status: 200
  Latency: 523ms
  Response: {"ip":"218.95.39.77"}

...

📊 Test 4: Pool statistics...
Total Proxies: 5
Available: 5 (100.00%)
...
Success Rate: 100.00%

==================================================
📝 Test Summary:
  Provider: ipzan ✅
  Proxies Fetched: 5 ✅
  Connectivity Tests: 3/3 passed

✅ Live test PASSED! Proxies are working.
```

## ⚠️ 注意事项

1. **需要真实的提取链接** - 测试会消耗代理商的配额
2. **可能需要网络代理** - 某些代理商API可能需要特定网络环境
3. **检查代理类型** - 确保`protocol`参数正确（IP赞：1=HTTP, 3=SOCKS5；51代理：port=1=HTTP, port=2=SOCKS5）
4. **防火墙设置** - 确保能访问代理商API和测试URL

## 🐛 常见问题

### Q1: `Failed to fetch proxies: timeout`
**A:** 检查网络连接，或增加超时时间

### Q2: `Failed to create provider: invalid extract URL`
**A:** 检查提取链接格式，必须包含 `format=json`

### Q3: `Request failed: proxy connection refused`
**A:** 代理IP可能已失效，或代理类型配置错误

### Q4: `api error (code=xxx)`
**A:** 检查API参数，可能是订单号、密钥错误或余额不足

## 📝 快速测试脚本

创建一个脚本文件 `test_ipzan.sh`:

```bash
#!/bin/bash
cd /Users/leo/Workspace/apps/proxypool

# 替换为你的真实提取链接
EXTRACT_URL='https://service.ipzan.com/core-extract?num=1&no=YOUR_NO&minute=3&format=json&protocol=3&pool=quality&mode=auth&secret=YOUR_SECRET'

go run live_test.go \
  -provider=ipzan \
  -url="$EXTRACT_URL" \
  -count=3 \
  -test='https://api.ipify.org?format=json'
```

运行：
```bash
chmod +x test_ipzan.sh
./test_ipzan.sh
```
