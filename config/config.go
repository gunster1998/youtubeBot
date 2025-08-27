package config

import (
	"os"
	"strings"
)

// Config содержит конфигурацию бота
type Config struct {
	TelegramToken string
	HTTPTimeout   int
	DownloadDir   string
}

// Load загружает конфигурацию из файла и переменных окружения
func Load(filename string) (*Config, error) {
	// Загружаем из файла .env
	if err := loadEnvFile(filename); err != nil {
		return nil, err
	}

	config := &Config{
		TelegramToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		HTTPTimeout:   30, // секунды
		DownloadDir:   "./downloads",
	}

	return config, nil
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
