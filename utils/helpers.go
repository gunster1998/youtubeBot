package utils

import (
	"log"
	"strings"
	"time"
)

// ExtractVideoID извлекает ID видео из YouTube URL
func ExtractVideoID(url string) string {
	// Извлекаем ID видео из URL
	if strings.Contains(url, "youtube.com/watch?v=") {
		parts := strings.Split(url, "v=")
		if len(parts) > 1 {
			videoID := strings.Split(parts[1], "&")[0]
			return videoID
		}
	} else if strings.Contains(url, "youtu.be/") {
		parts := strings.Split(url, "youtu.be/")
		if len(parts) > 1 {
			videoID := strings.Split(parts[1], "?")[0]
			return videoID
		}
	}
	return "unknown"
}

// IsValidYouTubeURL проверяет, является ли URL валидным YouTube URL
func IsValidYouTubeURL(url string) bool {
	return strings.Contains(url, "youtube.com/watch?v=") || 
		   strings.Contains(url, "youtu.be/") ||
		   strings.Contains(url, "youtube.com/embed/")
}

// SanitizeFilename очищает имя файла от недопустимых символов
func SanitizeFilename(filename string) string {
	// Заменяем недопустимые символы на подчеркивания
	invalidChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	result := filename
	
	for _, char := range invalidChars {
		result = strings.ReplaceAll(result, char, "_")
	}
	
	return result
}

// RetryWithBackoff выполняет функцию с повторными попытками и экспоненциальной задержкой
func RetryWithBackoff(operation func() error, maxRetries int, baseDelay time.Duration) error {
	var lastErr error
	
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Экспоненциальная задержка: 1s, 2s, 4s, 8s, 16s
			delay := baseDelay * time.Duration(1<<uint(attempt-1))
			log.Printf("🔄 Попытка %d/%d через %v...", attempt+1, maxRetries+1, delay)
			time.Sleep(delay)
		}
		
		err := operation()
		if err == nil {
			if attempt > 0 {
				log.Printf("✅ Операция успешна после %d попыток", attempt+1)
			}
			return nil
		}
		
		lastErr = err
		log.Printf("❌ Попытка %d/%d неудачна: %v", attempt+1, maxRetries+1, err)
	}
	
	log.Printf("💥 Все %d попыток исчерпаны", maxRetries+1)
	return lastErr
}
