package netx

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	xproxy "golang.org/x/net/proxy"
)

func getEnvBool(k string, def bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(k)))
	if v == "true" || v == "1" || v == "yes" { 
		return true 
	}
	if v == "false" || v == "0" || v == "no" { 
		return false 
	}
	return def
}

func hostInNoProxy(host string, noProxy string) bool {
	host = strings.ToLower(host)
	for _, token := range strings.Split(noProxy, ",") {
		token = strings.TrimSpace(strings.ToLower(token))
		if token == "" { 
			continue 
		}
		// точные хосты/домены
		if host == token || strings.HasSuffix(host, "."+token) {
			return true
		}
		// простые маски по подсетям
		if ip := net.ParseIP(host); ip != nil {
			_, cidr, err := net.ParseCIDR(token)
			if err == nil && cidr.Contains(ip) {
				return true
			}
		}
	}
	// дефолтные локальные
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() {
			return true
		}
	}
	return host == "localhost"
}

// Универсальный клиент: обходит прокси для локалок, иначе SOCKS5.
func NewHTTPClient() *http.Client {
	useProxy := getEnvBool("USE_PROXY", true)
	proxyURL := strings.TrimSpace(os.Getenv("PROXY_URL"))
	noProxy := os.Getenv("NO_PROXY")

	baseDialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	tr := &http.Transport{
		// Proxy не используем, чтобы не путать SOCKS с HTTP-прокси
		Proxy:             nil,
		DisableKeepAlives: false,
		ForceAttemptHTTP2: true,
		TLSClientConfig:   &tls.Config{MinVersion: tls.VersionTLS12},
	}

	tr.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, _, _ := net.SplitHostPort(address)
		// локальные/приватные — мимо прокси
		if hostInNoProxy(host, noProxy) || !useProxy || proxyURL == "" {
			return baseDialer.DialContext(ctx, network, address)
		}
		// SOCKS5 (удалённый резолв: оставляем hostname, не резолвим тут)
		socksAddr := strings.TrimPrefix(strings.TrimPrefix(proxyURL, "socks5h://"), "socks5://")
		d, err := xproxy.SOCKS5("tcp", socksAddr, nil, baseDialer)
		if err != nil {
			return nil, err
		}
		return d.Dial(network, address)
	}

	return &http.Client{
		Transport: tr,
		Timeout:   60 * time.Second,
	}
}

// Отдельный "прямой" клиент, если нужно гарантированно без прокси
func NewDirectHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy:           nil,
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
			DialContext:     (&net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
			ForceAttemptHTTP2: true,
		},
		Timeout: 60 * time.Second,
	}
}
