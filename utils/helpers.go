package utils

import (
	"strings"
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
