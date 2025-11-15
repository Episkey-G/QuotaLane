package util

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/proxy"
)

// CreateHTTPClient 创建支持代理的 HTTP 客户端
// 支持 SOCKS5 和 HTTP/HTTPS 代理
func CreateHTTPClient(proxyURL string, timeout time.Duration) (*http.Client, error) {
	if proxyURL == "" {
		return &http.Client{
			Timeout: timeout,
		}, nil
	}

	parsedProxy, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy URL: %w", err)
	}

	switch parsedProxy.Scheme {
	case "socks5":
		return createSOCKS5Client(parsedProxy, timeout)
	case "http", "https":
		return createHTTPProxyClient(parsedProxy, timeout)
	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %s", parsedProxy.Scheme)
	}
}

// createSOCKS5Client 创建 SOCKS5 代理客户端
func createSOCKS5Client(proxyURL *url.URL, timeout time.Duration) (*http.Client, error) {
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

	return &http.Client{
		Transport: &http.Transport{
			Dial: dialer.Dial,
		},
		Timeout: timeout,
	}, nil
}

// createHTTPProxyClient 创建 HTTP/HTTPS 代理客户端
func createHTTPProxyClient(proxyURL *url.URL, timeout time.Duration) (*http.Client, error) {
	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
		Timeout: timeout,
	}, nil
}
