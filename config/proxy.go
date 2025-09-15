package config

import (
	"net/http"
	"net/url"
	"os"
	"strings"
)

// ProxyConfig содержит настройки прокси
type ProxyConfig struct {
	UseProxy bool
	ProxyURL string
	NoProxy  []string
}

// LoadProxyConfig загружает конфигурацию прокси из переменных окружения
func LoadProxyConfig() *ProxyConfig {
	useProxy := strings.ToLower(os.Getenv("USE_PROXY")) == "true"
	proxyURL := os.Getenv("PROXY_URL")
	if proxyURL == "" {
		proxyURL = "socks5h://127.0.0.1:1080"
	}
	
	noProxy := os.Getenv("NO_PROXY")
	if noProxy == "" {
		noProxy = "localhost,127.0.0.1,172.16.0.0/12,192.168.0.0/16"
	}
	
	return &ProxyConfig{
		UseProxy: useProxy,
		ProxyURL: proxyURL,
		NoProxy:  strings.Split(noProxy, ","),
	}
}

// CreateHTTPClient создает HTTP клиент с настройками прокси
func (p *ProxyConfig) CreateHTTPClient() *http.Client {
	client := &http.Client{}
	
	if !p.UseProxy {
		return client
	}
	
	// Парсим URL прокси
	proxyURL, err := url.Parse(p.ProxyURL)
	if err != nil {
		// Если не удалось распарсить, возвращаем обычный клиент
		return client
	}
	
	// Создаем транспорт с прокси
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
	}
	
	client.Transport = transport
	return client
}

// GetProxyArgs возвращает аргументы прокси для yt-dlp
func (p *ProxyConfig) GetProxyArgs() []string {
	if !p.UseProxy {
		return []string{}
	}
	
	return []string{"--proxy", p.ProxyURL}
}

// GetCurlProxyArgs возвращает аргументы прокси для curl
func (p *ProxyConfig) GetCurlProxyArgs() []string {
	if !p.UseProxy {
		return []string{}
	}
	
	return []string{"--proxy", p.ProxyURL}
}

// ShouldProxy проверяет, нужно ли проксировать указанный хост
func (p *ProxyConfig) ShouldProxy(host string) bool {
	if !p.UseProxy {
		return false
	}
	
	// Проверяем исключения
	for _, noProxy := range p.NoProxy {
		noProxy = strings.TrimSpace(noProxy)
		if strings.Contains(host, noProxy) {
			return false
		}
	}
	
	return true
}

// GetEnvironmentVariables возвращает переменные окружения для прокси
func (p *ProxyConfig) GetEnvironmentVariables() map[string]string {
	env := make(map[string]string)
	
	if p.UseProxy {
		env["ALL_PROXY"] = p.ProxyURL
		env["HTTP_PROXY"] = p.ProxyURL
		env["HTTPS_PROXY"] = p.ProxyURL
		env["NO_PROXY"] = strings.Join(p.NoProxy, ",")
	}
	
	return env
}


