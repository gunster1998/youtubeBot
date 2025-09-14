package config

import (
	"os"
	"strings"
)

// Config содержит конфигурацию бота
type Config struct {
	TelegramToken string
	TelegramAPI   string // URL локального сервера Telegram API
	HTTPTimeout   int
	DownloadDir   string
	MaxFileSize   int64 // Максимальный размер файла в байтах (0 = без ограничений)
	Proxy         *ProxyConfig // Настройки прокси
}

// Load загружает конфигурацию из файла и переменных окружения
func Load(filename string) (*Config, error) {
	// Загружаем из файла .env
	if err := loadEnvFile(filename); err != nil {
		return nil, err
	}

	config := &Config{
		TelegramToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramAPI:   getEnvOrDefault("TELEGRAM_API_URL", "http://127.0.0.1:8081"),
		HTTPTimeout:   60, // увеличиваем таймаут для больших файлов
		DownloadDir:   "./downloads",
		MaxFileSize:   0, // 0 = без ограничений
		Proxy:         LoadProxyConfig(), // Загружаем настройки прокси
	}

	return config, nil
}

// getEnvOrDefault возвращает значение переменной окружения или значение по умолчанию
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
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
