package services

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// UniversalService универсальный сервис для работы с разными платформами
type UniversalService struct {
	downloadDir    string
	platformDetector *PlatformDetector
}

// NewUniversalService создает новый универсальный сервис
func NewUniversalService(downloadDir string) *UniversalService {
	return &UniversalService{
		downloadDir:    downloadDir,
		platformDetector: NewPlatformDetector(),
	}
}

// GetVideoFormats получает доступные форматы для любой платформы
func (us *UniversalService) GetVideoFormats(url string) ([]VideoFormat, error) {
	// Определяем платформу
	platformInfo := us.platformDetector.DetectPlatform(url)
	us.platformDetector.LogPlatformInfo(platformInfo, url)
	
	if !platformInfo.Supported {
		return nil, fmt.Errorf("платформа %s не поддерживается", platformInfo.DisplayName)
	}
	
	// Добавляем аргументы для получения форматов
	formatArgs := []string{
		"--list-formats",
		"--no-playlist",
		"--no-check-certificates",
	}
	
	// Добавляем аргументы прокси
	proxyArgs := getProxyArgs()
	
	// Объединяем все аргументы (без дублирования)
	allArgs := append(formatArgs, proxyArgs...)
	allArgs = append(allArgs, url)
	
	// Выполняем команду yt-dlp
	cmd := exec.Command(getYtDlpPath(), allArgs...)
	log.Printf("🚀 Выполняю команду для %s: %s", platformInfo.DisplayName, strings.Join(cmd.Args, " "))
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("❌ Ошибка yt-dlp для %s: %s", platformInfo.DisplayName, string(output))
		return nil, fmt.Errorf("ошибка получения форматов для %s: %v", platformInfo.DisplayName, err)
	}
	
	// Парсим форматы
	formats, err := us.parseVideoFormats(string(output), platformInfo.Type)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга форматов для %s: %v", platformInfo.DisplayName, err)
	}
	
	log.Printf("📊 Найдено %d форматов для %s", len(formats), platformInfo.DisplayName)
	return formats, nil
}

// DownloadVideoWithFormat скачивает видео в конкретном формате
func (us *UniversalService) DownloadVideoWithFormat(url, formatID string) (string, error) {
	// Определяем платформу
	platformInfo := us.platformDetector.DetectPlatform(url)
	if !platformInfo.Supported {
		return "", fmt.Errorf("платформа %s не поддерживается", platformInfo.DisplayName)
	}
	
	// Создаем папку для загрузок если не существует
	if err := os.MkdirAll(us.downloadDir, 0755); err != nil {
		return "", fmt.Errorf("не удалось создать папку для загрузок: %v", err)
	}
	
	// Добавляем специфичные аргументы для скачивания
	downloadArgs := []string{
		"--format", formatID,
		"--output", filepath.Join(us.downloadDir, "%(id)s_" + formatID + ".%(ext)s"),
		"--no-playlist",
		"--no-check-certificates",
		"--max-filesize", "2G",
		"--socket-timeout", "60",
		"--retries", "5",
	}
	
	// Если это аудиоформат, принудительно конвертируем в MP3
	// Проверяем по ID формата - если содержит "drc", "audio", "webm" или другие аудио ID, это аудио
	if strings.Contains(formatID, "drc") || strings.Contains(formatID, "audio") || strings.Contains(formatID, "bestaudio") || strings.Contains(formatID, "webm") {
		downloadArgs = append(downloadArgs, "--extract-audio", "--audio-format", "mp3", "--audio-quality", "0")
		log.Printf("🎵 Обнаружен аудиоформат %s, принудительно конвертирую в MP3", formatID)
	}
	
	// Добавляем аргументы прокси
	proxyArgs := getProxyArgs()
	
	// Объединяем все аргументы (без дублирования)
	allArgs := append(downloadArgs, proxyArgs...)
	allArgs = append(allArgs, url)
	
	// Выполняем команду yt-dlp
	cmd := exec.Command(getYtDlpPath(), allArgs...)
	log.Printf("🚀 Скачиваю %s: %s", platformInfo.DisplayName, strings.Join(cmd.Args, " "))
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("❌ Ошибка скачивания %s: %s", platformInfo.DisplayName, string(output))
		return "", fmt.Errorf("ошибка скачивания для %s: %v", platformInfo.DisplayName, err)
	}
	
	// Ищем скачанный файл
	videoFile, err := us.findDownloadedFile(url, platformInfo, formatID)
	if err != nil {
		return "", fmt.Errorf("не найден скачанный файл для %s: %v", platformInfo.DisplayName, err)
	}
	
	log.Printf("✅ Файл скачан для %s: %s", platformInfo.DisplayName, videoFile)
	return videoFile, nil
}

// parseVideoFormats парсит вывод yt-dlp для любой платформы
func (us *UniversalService) parseVideoFormats(output string, platformType PlatformType) ([]VideoFormat, error) {
	log.Printf("📋 Парсинг форматов для %s", platformType)
	
	var allFormats []VideoFormat
	lines := strings.Split(output, "\n")
	
	startParsing := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Начинаем парсинг после заголовка
		if strings.Contains(line, "Available formats") || strings.Contains(line, "ID  EXT") || 
		   strings.Contains(line, "ID EXT") || strings.Contains(line, "format code") {
			startParsing = true
			continue
		}
		
		// Пропускаем разделители
		if strings.Contains(line, "---") {
			continue
		}
		
		// Парсим строки с форматами
		if startParsing && len(strings.Fields(line)) >= 4 {
			parts := strings.Fields(line)
			
			// Проверяем, что это действительно формат
			if len(parts) >= 4 && (parts[1] == "mp4" || parts[1] == "webm" || parts[1] == "audio" || 
			   strings.Contains(parts[1], "video") || strings.Contains(parts[1], "audio")) {
				
				// Если это webm аудио - помечаем как audio для конвертации в MP3
				if parts[1] == "webm" && strings.Contains(strings.ToLower(line), "audio") {
					parts[1] = "audio" // Принудительно меняем на audio для конвертации
				}
				
				format := VideoFormat{
					ID:         parts[0],
					Extension:  parts[1],
					Resolution: parts[2],
					FPS:        parts[3],
					HasAudio:   strings.Contains(strings.ToLower(line), "audio") || parts[1] == "audio",
				}
				
				// Извлекаем размер файла если есть
				for i := 4; i < len(parts); i++ {
					if strings.Contains(parts[i], "MiB") || strings.Contains(parts[i], "GiB") || 
					   strings.Contains(parts[i], "KiB") || strings.Contains(parts[i], "B") {
						format.FileSize = parts[i]
						break
					}
				}
				
				allFormats = append(allFormats, format)
			}
		}
	}
	
	// Фильтруем форматы совместимые с Telegram
	return us.filterTelegramCompatibleFormats(allFormats), nil
}

// filterTelegramCompatibleFormats фильтрует форматы совместимые с Telegram
func (us *UniversalService) filterTelegramCompatibleFormats(formats []VideoFormat) []VideoFormat {
	var compatible []VideoFormat
	
	for _, format := range formats {
		// Telegram поддерживает MP4, MOV, MP3, M4A, OGG (webm конвертируется в mp3)
		if format.Extension == "mp4" || format.Extension == "mov" || format.Extension == "audio" {
			// Проверяем размер файла (максимум 2GB для Telegram)
			if !us.isFileTooLarge(format.FileSize, 2048) { // 2GB в MB
				compatible = append(compatible, format)
			}
		}
	}
	
	return compatible
}

// isFileTooLarge проверяет, превышает ли файл максимальный размер
func (us *UniversalService) isFileTooLarge(fileSize string, maxSizeMB int) bool {
	if fileSize == "" {
		return false // Если размер неизвестен, не блокируем
	}
	
	// Парсим размер файла
	if strings.Contains(fileSize, "MiB") {
		sizeStr := strings.Replace(fileSize, "MiB", "", 1)
		if size, err := strconv.ParseFloat(sizeStr, 64); err == nil {
			return size > float64(maxSizeMB)
		}
	}
	
	if strings.Contains(fileSize, "GiB") {
		sizeStr := strings.Replace(fileSize, "GiB", "", 1)
		if size, err := strconv.ParseFloat(sizeStr, 64); err == nil {
			return size*1024 > float64(maxSizeMB) // Конвертируем GB в MB
		}
	}
	
	return false
}

// findDownloadedFile ищет скачанный файл
func (us *UniversalService) findDownloadedFile(url string, platformInfo *PlatformInfo, formatID string) (string, error) {
	files, err := os.ReadDir(us.downloadDir)
	if err != nil {
		return "", fmt.Errorf("не удалось прочитать папку загрузок: %v", err)
	}
	
	// Ищем файл с ID видео и formatID
	var videoFile string
	expectedPattern := platformInfo.VideoID + "_" + formatID
	for _, file := range files {
		if !file.IsDir() && !strings.HasSuffix(file.Name(), ".webp") {
			// Проверяем, что файл содержит ID видео и formatID
			if strings.Contains(file.Name(), expectedPattern) {
				videoFile = filepath.Join(us.downloadDir, file.Name())
				log.Printf("🎯 Найден файл для %s %s (формат %s): %s", platformInfo.Icon, platformInfo.DisplayName, formatID, file.Name())
				break
			}
		}
	}
	
	if videoFile == "" {
		return "", fmt.Errorf("не найден скачанный файл для %s %s", platformInfo.Icon, platformInfo.DisplayName)
	}
	
	return videoFile, nil
}

// CheckYtDlp проверяет доступность yt-dlp
func (us *UniversalService) CheckYtDlp() error {
	cmd := exec.Command(getYtDlpPath(), "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("yt-dlp не найден: %v", err)
	}
	
	log.Printf("✅ yt-dlp доступен: %s", strings.TrimSpace(string(output)))
	return nil
}

// CheckNetwork проверяет сетевое подключение
func (us *UniversalService) CheckNetwork() error {
	// Простая проверка доступности интернета
	cmd := exec.Command("ping", "-c", "1", "8.8.8.8")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("нет подключения к интернету")
	}
	
	return nil
}

// GetSupportedPlatforms возвращает список поддерживаемых платформ
func (us *UniversalService) GetSupportedPlatforms() []PlatformInfo {
	return us.platformDetector.GetSupportedPlatforms()
}

// IsValidURL проверяет, является ли URL валидным
func (us *UniversalService) IsValidURL(url string) bool {
	return us.platformDetector.IsValidURL(url)
}

// GetPlatformInfo возвращает информацию о платформе по URL
func (us *UniversalService) GetPlatformInfo(url string) *PlatformInfo {
	return us.platformDetector.DetectPlatform(url)
}
