package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"strconv"
)

// VideoFormat представляет формат видео
type VideoFormat struct {
	ID         string
	Extension  string
	Resolution string
	FPS        string
	HasAudio   bool
	FileSize   string
}

// YouTubeService предоставляет методы для работы с YouTube
type YouTubeService struct {
	downloadDir string
}

// NewYouTubeService создает новый экземпляр YouTubeService
func NewYouTubeService(downloadDir string) *YouTubeService {
	return &YouTubeService{
		downloadDir: downloadDir,
	}
}

// GetVideoFormats получает список доступных форматов видео
func (s *YouTubeService) GetVideoFormats(url string) ([]VideoFormat, error) {
	log.Printf("🔍 Получение форматов для: %s", url)

	// Используем --list-formats для получения списка форматов
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "yt-dlp",
		"--list-formats",
		"--no-playlist",
		"--no-check-certificates",
		url)

	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("таймаут получения форматов (15 сек)")
		}
		return nil, fmt.Errorf("ошибка yt-dlp: %v", err)
	}

	log.Printf("📋 Получен вывод yt-dlp")

	// Парсим вывод yt-dlp
	var allFormats []VideoFormat
	lines := strings.Split(string(output), "\n")

	startParsing := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Пропускаем пустые строки
		if line == "" {
			continue
		}

		// Начинаем парсинг после строки "Available formats for"
		if strings.Contains(line, "Available formats for") {
			startParsing = true
			continue
		}

		// Пропускаем заголовки и разделители
		if strings.Contains(line, "ID  EXT") || strings.Contains(line, "---") {
			continue
		}

		// Парсим только если начали и строка содержит ID
		if startParsing && regexp.MustCompile(`^\d+`).MatchString(line) {
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				format := VideoFormat{
					ID:         parts[0],
					Extension:  parts[1],
					Resolution: parts[2],
					FPS:        parts[3],
					HasAudio:   !strings.Contains(line, "video only"),
				}

				// Извлекаем размер файла если есть
				if len(parts) >= 6 && parts[5] != "~" {
					format.FileSize = parts[5]
				}

				allFormats = append(allFormats, format)
				log.Printf("📹 Найден формат: %s %s %s (аудио: %v)",
					format.ID, format.Resolution, format.Extension, format.HasAudio)
			}
		}
	}

	// Фильтруем форматы для совместимости с Telegram
	telegramFormats := s.filterTelegramCompatibleFormats(allFormats)

	log.Printf("📊 Найдено %d форматов, %d совместимых с Telegram", len(allFormats), len(telegramFormats))
	return telegramFormats, nil
}

// filterTelegramCompatibleFormats фильтрует форматы для совместимости с Telegram
func (s *YouTubeService) filterTelegramCompatibleFormats(formats []VideoFormat) []VideoFormat {
	var compatible []VideoFormat

	for _, format := range formats {
		// Telegram поддерживает только определенные форматы
		if s.isTelegramCompatible(format) {
			compatible = append(compatible, format)
		}
	}

	return compatible
}

// isTelegramCompatible проверяет совместимость формата с Telegram
func (s *YouTubeService) isTelegramCompatible(format VideoFormat) bool {
	// Telegram поддерживает только MP4 и MOV
	if format.Extension != "mp4" && format.Extension != "mov" {
		return false
	}

	// Должен быть видео+аудио поток (не только видео)
	if !format.HasAudio {
		return false
	}

	// Пропускаем слишком низкие разрешения
	if format.Resolution == "48x27" || format.Resolution == "80x45" ||
		format.Resolution == "160x90" || format.Resolution == "320x180" {
		return false
	}

	// Пропускаем слишком высокие разрешения (Telegram ограничивает размер файла)
	if strings.Contains(format.Resolution, "4K") || strings.Contains(format.Resolution, "8K") {
		return false
	}

	// Проверяем размер файла (Telegram ограничивает до 50MB)
	if format.FileSize != "" {
		// Пример: "4.33MiB" -> проверяем что не слишком большой
		if strings.Contains(format.FileSize, "GiB") {
			return false
		}
	}

	return true
}

// isFileSizeTooLarge проверяет, превышает ли размер файла лимит Telegram (50MB)
func (s *YouTubeService) isFileSizeTooLarge(fileSize string) bool {
	// Telegram ограничивает размер файла до 50MB
	const maxSizeMB = 50
	
	// Парсим размер файла (например: "52.91MiB", "1.2GiB", "500KiB")
	fileSize = strings.TrimSpace(fileSize)
	
	// Если размер в гигабайтах - точно превышает лимит
	if strings.Contains(fileSize, "GiB") {
		return true
	}
	
	// Если размер в мегабайтах - проверяем значение
	if strings.Contains(fileSize, "MiB") {
		// Извлекаем числовое значение
		sizeStr := strings.Replace(fileSize, "MiB", "", 1)
		if size, err := strconv.ParseFloat(sizeStr, 64); err == nil {
			return size > float64(maxSizeMB)
		}
	}
	
	// Если размер в килобайтах - точно не превышает
	if strings.Contains(fileSize, "KiB") {
		return false
	}
	
	// Если размер в байтах - проверяем
	if strings.Contains(fileSize, "B") && !strings.Contains(fileSize, "KiB") && !strings.Contains(fileSize, "MiB") && !strings.Contains(fileSize, "GiB") {
		sizeStr := strings.Replace(fileSize, "B", "", 1)
		if size, err := strconv.ParseFloat(sizeStr, 64); err == nil {
			return size > float64(maxSizeMB*1024*1024) // 50MB в байтах
		}
	}
	
	// Если не можем распарсить - пропускаем (лучше перестраховаться)
	log.Printf("⚠️ Не удалось распарсить размер файла: %s", fileSize)
	return true
}

// DownloadVideo скачивает видео с YouTube
func (s *YouTubeService) DownloadVideo(url string) (string, error) {
	// Создаем папку для загрузок если не существует
	if err := os.MkdirAll(s.downloadDir, 0755); err != nil {
		return "", fmt.Errorf("не удалось создать папку для загрузок: %v", err)
	}

	log.Printf("💾 Скачивание видео: %s", url)

	// Простая команда yt-dlp для скачивания лучшего MP4 формата
	cmd := exec.Command("yt-dlp",
		"--format", "best[ext=mp4]/best", // Лучший MP4 или любой лучший
		"--output", filepath.Join(s.downloadDir, "%(id)s.%(ext)s"), // Имя файла по ID
		"--no-playlist",           // Только одно видео
		"--no-check-certificates", // Ускоряем процесс
		url)

	log.Printf("🚀 Выполняю команду: %s", strings.Join(cmd.Args, " "))

	// Запускаем команду
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("❌ Ошибка yt-dlp: %s", string(output))
		return "", fmt.Errorf("ошибка yt-dlp: %v", err)
	}

	log.Printf("✅ yt-dlp выполнен успешно: %s", string(output))

	// Ищем скачанный файл
	videoFile, err := s.findDownloadedFile()
	if err != nil {
		return "", err
	}

	return videoFile, nil
}

// DownloadVideoWithFormat скачивает видео в конкретном формате
func (s *YouTubeService) DownloadVideoWithFormat(videoID, formatID string) (string, error) {
	// Создаем папку для загрузок если не существует
	if err := os.MkdirAll(s.downloadDir, 0755); err != nil {
		return "", fmt.Errorf("не удалось создать папку для загрузок: %v", err)
	}

	url := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
	log.Printf("💾 Скачивание видео %s в формате %s", videoID, formatID)

	// Команда yt-dlp для скачивания в конкретном формате
	cmd := exec.Command("yt-dlp",
		"--format", formatID,
		"--output", filepath.Join(s.downloadDir, "%(id)s.%(ext)s"),
		"--no-playlist",
		"--no-check-certificates",
		url)

	log.Printf("🚀 Выполняю команду: %s", strings.Join(cmd.Args, " "))

	// Запускаем команду
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("❌ Ошибка yt-dlp: %s", string(output))
		return "", fmt.Errorf("ошибка yt-dlp: %v", err)
	}

	log.Printf("✅ yt-dlp выполнен успешно: %s", string(output))

	// Ищем скачанный файл
	videoFile, err := s.findDownloadedFile()
	if err != nil {
		return "", err
	}

	return videoFile, nil
}

// findDownloadedFile ищет скачанный видео файл
func (s *YouTubeService) findDownloadedFile() (string, error) {
	files, err := os.ReadDir(s.downloadDir)
	if err != nil {
		return "", fmt.Errorf("не удалось прочитать папку загрузок: %v", err)
	}

	// Ищем любой видео файл
	var videoFile string
	for _, file := range files {
		if !file.IsDir() && !strings.HasSuffix(file.Name(), ".webp") {
			videoFile = filepath.Join(s.downloadDir, file.Name())
			break
		}
	}

	if videoFile == "" {
		return "", fmt.Errorf("не найден скачанный видео файл")
	}

	return videoFile, nil
}

// CheckYtDlp проверяет наличие yt-dlp в системе
func (s *YouTubeService) CheckYtDlp() error {
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return fmt.Errorf("yt-dlp не найден в системе. Установите его: brew install yt-dlp")
	}
	return nil
}
