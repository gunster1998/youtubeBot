package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

func main() {
	fmt.Println("🧪 Самопроверка YouTube Bot с прокси")
	fmt.Println("====================================")

	// Загружаем переменные окружения
	loadEnvFile(".env")

	// Проверяем настройки прокси
	useProxy := strings.ToLower(os.Getenv("USE_PROXY")) == "true"
	proxyURL := os.Getenv("PROXY_URL")
	if proxyURL == "" {
		proxyURL = "socks5h://127.0.0.1:1080"
	}

	fmt.Printf("USE_PROXY = %t\n", useProxy)
	fmt.Printf("PROXY_URL = %s\n", proxyURL)

	// Тест 1: HTTP клиент с прокси
	fmt.Println("\n🌐 Тест 1: HTTP клиент с прокси")
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	if useProxy {
		// Создаем транспорт с прокси
		proxyURLParsed, err := parseProxyURL(proxyURL)
		if err != nil {
			fmt.Printf("❌ Ошибка парсинга прокси URL: %v\n", err)
		} else {
			transport := &http.Transport{
				Proxy: http.ProxyURL(proxyURLParsed),
			}
			client.Transport = transport
			fmt.Printf("✅ HTTP клиент настроен с прокси: %s\n", proxyURL)
		}
	} else {
		fmt.Println("ℹ️ Прокси отключен, используется прямое подключение")
	}

	// Тест подключения к Google
	resp, err := client.Get("https://www.google.com")
	if err != nil {
		fmt.Printf("❌ Ошибка подключения к Google: %v\n", err)
	} else {
		resp.Body.Close()
		fmt.Printf("✅ Подключение к Google: %d\n", resp.StatusCode)
	}

	// Тест 2: yt-dlp с прокси
	fmt.Println("\n🎬 Тест 2: yt-dlp с прокси")
	cmd := []string{"yt-dlp", "-s", "https://www.youtube.com/watch?v=dQw4w9WgXcQ"}
	
	if useProxy {
		cmd = append([]string{"yt-dlp", "--proxy", proxyURL, "-s"}, cmd[2:]...)
		fmt.Printf("yt-dlp dry-run cmd: %s\n", strings.Join(cmd, " "))
	} else {
		fmt.Printf("yt-dlp dry-run cmd: %s\n", strings.Join(cmd, " "))
	}

	output, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
	if err != nil {
		fmt.Printf("❌ yt-dlp failed: %v\n", err)
		fmt.Printf("Output: %s\n", string(output))
	} else {
		fmt.Printf("✅ yt-dlp ok, bytes: %d\n", len(output))
	}

	// Тест 3: curl с прокси
	fmt.Println("\n🔗 Тест 3: curl с прокси")
	curlCmd := []string{"curl", "-s", "--connect-timeout", "10", "--max-time", "30"}
	
	if useProxy {
		curlCmd = append(curlCmd, "--proxy", proxyURL)
	}
	curlCmd = append(curlCmd, "https://www.youtube.com")

	curlOutput, err := exec.Command(curlCmd[0], curlCmd[1:]...).CombinedOutput()
	if err != nil {
		fmt.Printf("❌ curl failed: %v\n", err)
	} else {
		fmt.Printf("✅ curl ok, bytes: %d\n", len(curlOutput))
	}

	// Тест 4: Проверка переменных окружения
	fmt.Println("\n🔧 Тест 4: Переменные окружения")
	envVars := []string{"USE_PROXY", "PROXY_URL", "NO_PROXY", "ALL_PROXY", "HTTP_PROXY", "HTTPS_PROXY"}
	for _, envVar := range envVars {
		value := os.Getenv(envVar)
		if value != "" {
			fmt.Printf("✅ %s = %s\n", envVar, value)
		} else {
			fmt.Printf("ℹ️ %s не установлена\n", envVar)
		}
	}

	fmt.Println("\n🎉 Самопроверка завершена!")
}

// loadEnvFile загружает переменные окружения из файла
func loadEnvFile(filename string) error {
	content, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			os.Setenv(key, value)
		}
	}

	return nil
}

// parseProxyURL парсит URL прокси
func parseProxyURL(proxyURL string) (*url.URL, error) {
	return url.Parse(proxyURL)
}
