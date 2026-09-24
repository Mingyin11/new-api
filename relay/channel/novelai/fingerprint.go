package novelai

import (
	"net/http"
	"strings"
)

const (
	DefaultServerUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
)

// SanitizeAndSpoofHeaders 剥离下游客户端的所有设备指纹和真实 IP 信息，并统一以服务端设备指纹调用上游
func SanitizeAndSpoofHeaders(req *http.Request, token string) {
	if req == nil {
		return
	}

	// 1. 彻底清除下游客户端透传特征头
	headersToDrop := []string{
		"User-Agent",
		"X-Forwarded-For",
		"X-Real-Ip",
		"Cf-Connecting-Ip",
		"True-Client-Ip",
		"X-Client-Ip",
		"Forwarded",
		"Sec-Ch-Ua",
		"Sec-Ch-Ua-Mobile",
		"Sec-Ch-Ua-Platform",
		"Sec-Ch-Ua-Platform-Version",
		"Sec-Ch-Ua-Model",
		"Sec-Ch-Ua-Arch",
		"Sec-Ch-Ua-Bitness",
		"Sec-Fetch-Dest",
		"Sec-Fetch-Mode",
		"Sec-Fetch-Site",
		"Sec-Fetch-User",
		"Origin",
		"Referer",
		"Accept-Language",
		"X-Request-Id",
		"X-Correlation-Id",
	}

	for _, h := range headersToDrop {
		req.Header.Del(h)
	}

	// 2. 注入统一的服务端设备指纹
	req.Header.Set("User-Agent", DefaultServerUserAgent)
	cleanToken := strings.TrimSpace(token)
	if !strings.HasPrefix(cleanToken, "Bearer ") {
		cleanToken = "Bearer " + cleanToken
	}
	req.Header.Set("Authorization", cleanToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Sec-Ch-Ua", `"Google Chrome";v="131", "Chromium";v="131", "Not_A Brand";v="24"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
}
