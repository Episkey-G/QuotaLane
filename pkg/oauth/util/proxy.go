package util

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/proxy"
)

// CreateHTTPClient 创建支持代理的 HTTP 客户端
// 支持 SOCKS5 和 HTTP/HTTPS 代理
// 包含完善的连接池配置和超时设置
func CreateHTTPClient(proxyURL string, timeout time.Duration) (*http.Client, error) {
	// 默认超时时间
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// 创建基础 Transport 配置
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	// 配置代理
	if proxyURL != "" {
		parsedProxy, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %w", err)
		}

		switch parsedProxy.Scheme {
		case "socks5", "socks5h":
			// SOCKS5 代理
			dialer, err := createSOCKS5Dialer(parsedProxy)
			if err != nil {
				return nil, fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
			}
			transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			}

		case "http", "https":
			// HTTP/HTTPS 代理
			transport.Proxy = http.ProxyURL(parsedProxy)

		default:
			return nil, fmt.Errorf("unsupported proxy scheme: %s (supported: socks5, http, https)", parsedProxy.Scheme)
		}
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}, nil
}

// createSOCKS5Dialer 创建 SOCKS5 代理 Dialer
func createSOCKS5Dialer(proxyURL *url.URL) (proxy.Dialer, error) {
	var auth *proxy.Auth
	if proxyURL.User != nil {
		password, _ := proxyURL.User.Password()
		auth = &proxy.Auth{
			User:     proxyURL.User.Username(),
			Password: password,
		}
	}

	dialer, err := proxy.SOCKS5("tcp", proxyURL.Host, auth, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
	}

	return dialer, nil
}
