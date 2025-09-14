package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"strconv"
	
	"youtubeBot/utils"
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

// VideoMetadata представляет метаданные видео
type VideoMetadata struct {
	Title       string
	Author      string
	Duration    string
	Views       string
	Description string
	Thumbnail   string
	UploadDate  string
	OriginalURL string
}

// YouTubeService предоставляет методы для работы с YouTube
type YouTubeService struct {
	downloadDir string
}

// getYtDlpPath возвращает путь к yt-dlp
func getYtDlpPath() string {
	// Сначала проверяем новый путь
	if _, err := exec.LookPath("/usr/local/bin/yt-dlp"); err == nil {
		return "/usr/local/bin/yt-dlp"
	}
	
	// Если не найден, проверяем старый путь
	if _, err := exec.LookPath("yt-dlp"); err == nil {
		return "yt-dlp"
	}
	
	return "/usr/local/bin/yt-dlp" // По умолчанию
}

// getProxyArgs возвращает аргументы прокси для yt-dlp
func getProxyArgs() []string {
	var args []string
	
	// Проверяем USE_PROXY флаг
	useProxy := strings.ToLower(os.Getenv("USE_PROXY")) == "true"
	if !useProxy {
		log.Printf("🌐 Прокси отключен (USE_PROXY=false)")
		return args
	}
	
	// Проверяем PROXY_URL (новый приоритетный способ)
	if proxyURL := os.Getenv("PROXY_URL"); proxyURL != "" {
		args = append(args, "--proxy", proxyURL)
		log.Printf("🌐 Используется PROXY_URL: %s", proxyURL)
	} else if allProxy := os.Getenv("ALL_PROXY"); allProxy != "" {
		args = append(args, "--proxy", allProxy)
		log.Printf("🌐 Используется ALL_PROXY: %s", allProxy)
	} else if httpProxy := os.Getenv("HTTP_PROXY"); httpProxy != "" {
		args = append(args, "--proxy", httpProxy)
		log.Printf("🌐 Используется HTTP_PROXY: %s", httpProxy)
	} else if httpsProxy := os.Getenv("HTTPS_PROXY"); httpsProxy != "" {
		args = append(args, "--proxy", httpsProxy)
		log.Printf("🌐 Используется HTTPS_PROXY: %s", httpsProxy)
	} else if socksProxy := os.Getenv("SOCKS_PROXY"); socksProxy != "" {
		args = append(args, "--proxy", socksProxy)
		log.Printf("🌐 Используется SOCKS_PROXY: %s", socksProxy)
	}
	
	// Добавляем анти-429 задержки для стабильности
	args = append(args, "--sleep-requests", "1")
	args = append(args, "--sleep-interval", "1")
	args = append(args, "--max-sleep-interval", "3")
	
	return args
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
	log.Printf("🚀 Запуск yt-dlp для анализа видео...")

	var formats []VideoFormat
	var lastErr error
	
	// Используем retry механизм для получения форматов
	err := utils.RetryWithBackoff(func() error {
		// Используем --list-formats для получения списка форматов
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		log.Printf("⏱️ Таймаут установлен на 120 секунд")

		// Получаем аргументы прокси
		proxyArgs := getProxyArgs()
		
		// Формируем команду с прокси (упрощаем для получения всех форматов)
		args := []string{
			"--list-formats",
			"--no-playlist",
			"--no-check-certificates",
			"--no-warnings",
			// Убираем --quiet для лучшего вывода
			// Убираем --extractor-args для получения всех форматов
		}
		
		// Добавляем аргументы прокси
		args = append(args, proxyArgs...)
		args = append(args, url)
		
		cmd := exec.CommandContext(ctx, getYtDlpPath(), args...)

		output, err := cmd.CombinedOutput()
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return fmt.Errorf("таймаут получения форматов (120 сек) - видео слишком большое или медленный интернет")
			}
			log.Printf("❌ yt-dlp ошибка: %v", err)
			log.Printf("📋 Вывод yt-dlp: %s", string(output))
			return fmt.Errorf("ошибка yt-dlp: %v", err)
		}
		
		// Парсим результат
		parsedFormats, parseErr := s.parseVideoFormats(string(output))
		if parseErr != nil {
			return parseErr
		}
		
		formats = parsedFormats
		return nil
	}, 3, 2*time.Second) // 3 попытки с базовой задержкой 2 секунды
	
	if err != nil {
		lastErr = err
		log.Printf("💥 Не удалось получить форматы после всех попыток: %v", err)
		return nil, lastErr
	}

	log.Printf("📊 Найдено %d форматов, %d совместимых с Telegram", len(formats), len(s.filterTelegramCompatibleFormats(formats)))
	return s.filterTelegramCompatibleFormats(formats), nil
}

// parseVideoFormats парсит вывод yt-dlp и возвращает список форматов
func (s *YouTubeService) parseVideoFormats(output string) ([]VideoFormat, error) {
	log.Printf("📋 Парсинг вывода yt-dlp")
	log.Printf("🔍 Сырой вывод yt-dlp:\n%s", output)

	// Парсим вывод yt-dlp
	var allFormats []VideoFormat
	lines := strings.Split(output, "\n")

	log.Printf("📊 Всего строк в выводе: %d", len(lines))
	
	startParsing := false
	headerFound := false

	for i, line := range lines {
		line = strings.TrimSpace(line)
		
		log.Printf("🔍 Строка %d: '%s'", i+1, line)

		// Пропускаем пустые строки
		if line == "" {
			continue
		}

		// Начинаем парсинг после строки "Available formats for" или заголовка таблицы
		if strings.Contains(line, "Available formats for") || strings.Contains(line, "ID  EXT") || 
		   strings.Contains(line, "ID EXT") || strings.Contains(line, "format code") {
			startParsing = true
			headerFound = true
			log.Printf("✅ Найден заголовок таблицы: '%s'", line)
			continue
		}

		// Пропускаем разделители
		if strings.Contains(line, "---") {
			continue
		}

			// Парсим строки с форматами (начинаются с ID)
		if startParsing && regexp.MustCompile(`^\d+`).MatchString(line) {
			parts := strings.Fields(line)
			log.Printf("🔍 Парсинг строки: %s (частей: %d)", line, len(parts))
			
			// Пропускаем строки, которые не являются видео/аудио форматами
			if len(parts) < 4 {
				log.Printf("⚠️ Строка слишком короткая для парсинга: %s (частей: %d)", line, len(parts))
				continue
			}
			
			// Проверяем, что это действительно формат (не служебная информация)
			if parts[1] == "audio" || parts[1] == "mp4" || parts[1] == "webm" || parts[1] == "mov" {
				log.Printf("✅ Найден формат: %s %s %s", parts[0], parts[1], parts[2])
			} else {
				log.Printf("⏭️ Пропускаю неформатную строку: %s", line)
				continue
			}
			
			if len(parts) >= 4 {
				// Пропускаем storyboard форматы
				if strings.HasPrefix(parts[0], "sb") {
					log.Printf("⏭️ Пропускаю storyboard формат: %s", parts[0])
					continue
				}

				// Определяем наличие аудио по колонке CH (каналы)
				hasAudio := false
				if len(parts) >= 5 {
					// Если в колонке CH есть число больше 0, значит есть аудио
					if channels, err := strconv.Atoi(parts[4]); err == nil && channels > 0 {
						hasAudio = true
						log.Printf("🎵 Найдены аудио каналы: %d", channels)
					}
				}
				
				// Дополнительная проверка по тексту
				if !hasAudio {
					hasAudio = !strings.Contains(line, "video only")
				}
				
				// Дополнительные проверки для YouTube
				if !hasAudio && strings.Contains(line, "mp4") {
					// Для MP4 форматов YouTube часто есть аудио
					if !strings.Contains(line, "video only") && !strings.Contains(line, "audio only") {
						hasAudio = true
						log.Printf("🎵 MP4 формат без 'video only' - считаю что есть аудио")
					}
				}
				
				// Проверяем, есть ли в строке "video only" - это означает что аудио НЕТ
				if strings.Contains(line, "video only") {
					hasAudio = false
					log.Printf("🔇 Найдено 'video only' - аудио отсутствует")
				}
				
				log.Printf("🔍 Анализ аудио для %s: hasAudio=%v, строка='%s'", parts[0], hasAudio, line)
				
				// Определяем тип формата
				formatType := parts[1] // EXT колонка
				log.Printf("🔍 Анализирую тип формата: '%s' для ID %s", formatType, parts[0])
				
				if formatType == "audio" {
					// Это аудио формат
					formatType = "audio"
					hasAudio = true
					log.Printf("🎵 Обнаружен аудио формат: ID %s", parts[0])
				} else if strings.Contains(line, "audio only") {
					// Альтернативная проверка для аудио
					formatType = "audio"
					hasAudio = true
					log.Printf("🎵 Обнаружен аудио формат (по тексту): ID %s", parts[0])
				} else if strings.Contains(line, "webm") && strings.Contains(line, "audio") {
					// WebM аудио формат - принудительно конвертируем в MP3
					formatType = "audio"
					hasAudio = true
					log.Printf("🎵 Обнаружен WebM аудио формат: ID %s - будет конвертирован в MP3", parts[0])
				}

				format := VideoFormat{
					ID:         parts[0],
					Extension:  formatType, // Используем определенный тип
					Resolution: parts[2],
					FPS:        parts[3],
					HasAudio:   hasAudio,
				}
				
				log.Printf("📝 Создана структура: ID=%s, Extension='%s', Resolution=%s, HasAudio=%v", 
					format.ID, format.Extension, format.Resolution, format.HasAudio)

				// Извлекаем размер файла если есть
				// Ищем размер файла в разных колонках (yt-dlp может менять порядок)
				format.FileSize = ""
				for i := 5; i < len(parts); i++ {
					if strings.Contains(parts[i], "MiB") || strings.Contains(parts[i], "GiB") || 
					   strings.Contains(parts[i], "KiB") || strings.Contains(parts[i], "B") {
						format.FileSize = parts[i]
						log.Printf("📏 Размер файла: %s (колонка %d)", parts[i], i)
						break
					}
				}
				
				if format.FileSize == "" {
					log.Printf("⚠️ Размер файла не найден в строке: %s", line)
				}

				// Фильтруем дублирующиеся форматы (после извлечения размера)
				if format.Extension == "audio" {
					// Для аудио: оставляем только лучшие качества (не дублирующиеся)
					isDuplicate := false
					for _, existing := range allFormats {
						if existing.Extension == "audio" {
							// Если размер одинаковый - это дубликат
							if existing.FileSize == format.FileSize {
								isDuplicate = true
								log.Printf("⏭️ Пропускаю дублирующийся аудио формат: %s (размер: %s)", format.ID, format.FileSize)
								break
							}
							// Если разрешение одинаковое - оставляем лучший (больший размер)
							if existing.Resolution == format.Resolution {
								if s.isBetterAudioQuality(format, existing) {
									// Заменяем худший на лучший
									log.Printf("🔄 Заменяю худший аудио формат %s на лучший %s для разрешения %s", 
										existing.ID, format.ID, format.Resolution)
									// Находим и заменяем в списке
									for i, f := range allFormats {
										if f.ID == existing.ID {
											allFormats[i] = format
											break
										}
									}
									goto nextFormat
								} else {
									// Пропускаем худший
									log.Printf("⏭️ Пропускаю худший аудио формат %s для разрешения %s", 
										format.ID, format.Resolution)
									goto nextFormat
								}
							}
						}
					}
					if isDuplicate {
						continue
					}
				} else {
					// ВРЕМЕННО: Показываем ВСЕ видео форматы для отладки
					// Позже можно будет включить фильтрацию по звуку обратно
					/*
					if !format.HasAudio {
						log.Printf("⏭️ Пропускаю видео без звука: %s (%s)", format.ID, format.Resolution)
						goto nextFormat
					}
					*/
					
					log.Printf("✅ Видео формат добавлен: %s (%s) - %s (аудио: %v)", 
						format.ID, format.Resolution, format.FileSize, format.HasAudio)
				}

				allFormats = append(allFormats, format)
				log.Printf("📹 Найден формат: %s %s %s (аудио: %v, размер: %s)",
					format.ID, format.Resolution, format.Extension, format.HasAudio, format.FileSize)
			nextFormat:
				continue
			} else {
				log.Printf("⚠️ Строка слишком короткая для парсинга: %s (частей: %d)", line, len(parts))
			}
		}
	}

	// Логируем все найденные форматы
	log.Printf("🔍 ВСЕ найденные форматы (до фильтрации):")
	var audioCount, videoWithAudioCount, videoWithoutAudioCount int
	for _, f := range allFormats {
		log.Printf("  - %s: %s %s (аудио: %v, размер: %s)", 
			f.ID, f.Resolution, f.Extension, f.HasAudio, f.FileSize)
		
		if f.Extension == "audio" {
			audioCount++
		} else if f.HasAudio {
			videoWithAudioCount++
		} else {
			videoWithoutAudioCount++
		}
	}
	log.Printf("📊 Статистика: %d аудио, %d видео со звуком, %d видео без звука", 
		audioCount, videoWithAudioCount, videoWithoutAudioCount)
	
	// Дополнительная отладка для видео форматов
	if videoWithAudioCount == 0 {
		log.Printf("⚠️ ВНИМАНИЕ: Не найдено видео форматов со звуком!")
		log.Printf("🔍 Проверяю все видео форматы:")
		for _, f := range allFormats {
			if f.Extension != "audio" {
				log.Printf("  🎥 %s: %s %s (аудио: %v, размер: %s)", 
					f.ID, f.Resolution, f.Extension, f.HasAudio, f.FileSize)
			}
		}
	}

	// Проверяем, что мы действительно нашли форматы
	if len(allFormats) == 0 {
		log.Printf("❌ КРИТИЧЕСКАЯ ОШИБКА: Не найдено ни одного формата!")
		log.Printf("🔍 Проверьте вывод yt-dlp выше")
		if !headerFound {
			log.Printf("❌ Заголовок таблицы форматов не найден!")
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

	log.Printf("🔍 Фильтрация %d форматов для совместимости с Telegram", len(formats))

	for _, format := range formats {
		log.Printf("🔍 Проверяю формат %s: %s %s (аудио: %v, размер: %s)", 
			format.ID, format.Resolution, format.Extension, format.HasAudio, format.FileSize)
		
		// Telegram поддерживает только определенные форматы
		if s.isTelegramCompatible(format) {
			compatible = append(compatible, format)
			log.Printf("✅ Формат %s прошел фильтрацию", format.ID)
		} else {
			log.Printf("❌ Формат %s не прошел фильтрацию", format.ID)
		}
	}

	log.Printf("📊 Результат фильтрации: %d из %d форматов совместимы с Telegram", len(compatible), len(formats))
	return compatible
}

// isTelegramCompatible проверяет совместимость формата с Telegram
func (s *YouTubeService) isTelegramCompatible(format VideoFormat) bool {
	// Разрешаем все аудио форматы (webm будет конвертирован в mp3)
	if format.Extension == "audio" {
		log.Printf("✅ Аудио формат %s совместим с Telegram: %s (размер: %s) - будет конвертирован в MP3", 
			format.ID, format.Resolution, format.FileSize)
		return true
	}
	
	// Для видео: только MP4 и MOV
	if format.Extension != "mp4" && format.Extension != "mov" {
		log.Printf("❌ Формат %s не поддерживается: %s", format.ID, format.Extension)
		return false
	}

	// Проверяем размер файла (максимум 2GB)
	if format.FileSize != "" {
		if s.isFileSizeTooLarge(format.FileSize) {
			log.Printf("📏 Формат %s превышает лимит 2GB: %s", format.ID, format.FileSize)
			return false
		}
	}

	log.Printf("✅ Формат %s совместим с Telegram: %s %s (размер: %s)", 
		format.ID, format.Resolution, format.Extension, format.FileSize)
	return true
}

// isFileSizeTooLarge проверяет, превышает ли размер файла лимит (2GB)
func (s *YouTubeService) isFileSizeTooLarge(fileSize string) bool {
	// Локальный сервер поддерживает файлы до 2GB
	const maxSizeMB = 2048 // 2GB в MB
	
	// Парсим размер файла (например: "≈301.82MiB", "52.91MiB", "1.2GiB", "500KiB")
	fileSize = strings.TrimSpace(fileSize)
	
	// Убираем символы ≈, ~, если есть
	fileSize = strings.TrimPrefix(fileSize, "≈")
	fileSize = strings.TrimPrefix(fileSize, "~")
	fileSize = strings.TrimSpace(fileSize)
	
	// Если размер в гигабайтах - проверяем значение
	if strings.Contains(fileSize, "GiB") {
		// Извлекаем числовое значение
		sizeStr := strings.Replace(fileSize, "GiB", "", 1)
		if size, err := strconv.ParseFloat(sizeStr, 64); err == nil {
			isTooLarge := size > 2.0 // Максимум 2GB
			log.Printf("📏 Размер в гигабайтах: %s (%.2f GB) - %s", fileSize, size, 
				func() string { if isTooLarge { return "превышает лимит 2GB" } else { return "в пределах лимита 2GB" } }())
			return isTooLarge
		}
	}
	
	// Если размер в мегабайтах - проверяем значение
	if strings.Contains(fileSize, "MiB") {
		// Извлекаем числовое значение
		sizeStr := strings.Replace(fileSize, "MiB", "", 1)
		if size, err := strconv.ParseFloat(sizeStr, 64); err == nil {
			isTooLarge := size > float64(maxSizeMB)
				log.Printf("📏 Размер в мегабайтах: %s (%.2f MB) - %s", fileSize, size, 
		func() string { if isTooLarge { return "превышает лимит 2GB" } else { return "в пределах лимита 2GB" } }())
	return isTooLarge
		}
	}
	
	// Если размер в килобайтах - точно не превышает
	if strings.Contains(fileSize, "KiB") {
		log.Printf("📏 Размер в килобайтах: %s - в пределах лимита", fileSize)
		return false
	}
	
	// Если размер в байтах - проверяем
	if strings.Contains(fileSize, "B") && !strings.Contains(fileSize, "KiB") && !strings.Contains(fileSize, "MiB") && !strings.Contains(fileSize, "GiB") {
		sizeStr := strings.Replace(fileSize, "B", "", 1)
		if size, err := strconv.ParseFloat(sizeStr, 64); err == nil {
			isTooLarge := size > float64(maxSizeMB*1024*1024) // 50MB в байтах
			log.Printf("📏 Размер в байтах: %s (%.0f B) - %s", fileSize, size, 
				func() string { if isTooLarge { return "превышает лимит" } else { return "в пределах лимита" } }())
			return isTooLarge
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

	// Получаем аргументы прокси
	proxyArgs := getProxyArgs()
	
	// Команда yt-dlp для скачивания лучшего MP4 формата (поддержка до 2GB)
	args := []string{
		"--format", "best[ext=mp4]/best", // Лучший MP4 или любой лучший
		"--output", filepath.Join(s.downloadDir, "%(id)s.%(ext)s"), // Имя файла по ID
		"--no-playlist",           // Только одно видео
		"--no-check-certificates", // Ускоряем процесс
		"--max-filesize", "2G",    // Максимальный размер файла 2GB
		"--socket-timeout", "60",  // Увеличенный таймаут для больших файлов
		"--retries", "5",          // Больше попыток для больших файлов
	}
	
	// Добавляем аргументы прокси
	args = append(args, proxyArgs...)
	args = append(args, url)
	
	// Добавляем timeout для команды
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, getYtDlpPath(), args...)

	log.Printf("🚀 Выполняю команду: %s", strings.Join(cmd.Args, " "))

	// Запускаем команду
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("❌ Ошибка yt-dlp: %s", string(output))
		return "", fmt.Errorf("ошибка yt-dlp: %v", err)
	}

	log.Printf("✅ yt-dlp выполнен успешно: %s", string(output))

	// Ищем скачанный файл для конкретного видео (без формата для старого метода)
	videoFile, err := s.findDownloadedFileOld(url)
	if err != nil {
		return "", err
	}

	return videoFile, nil
}

// DownloadVideoWithFormat скачивает видео в конкретном формате
func (s *YouTubeService) DownloadVideoWithFormat(videoURL, formatID string) (string, error) {
	// Создаем папку для загрузок если не существует
	if err := os.MkdirAll(s.downloadDir, 0755); err != nil {
		return "", fmt.Errorf("не удалось создать папку для загрузок: %v", err)
	}

	// Очищаем только файлы для конкретного видео ID и формата
	if err := s.cleanVideoFiles(videoURL, formatID); err != nil {
		log.Printf("⚠️ Не удалось очистить файлы для видео: %v", err)
	}

	log.Printf("💾 Скачивание видео %s в формате %s + аудио", videoURL, formatID)

	var videoFile string
	var lastErr error
	
	// Используем retry механизм для скачивания
	err := utils.RetryWithBackoff(func() error {
		// Получаем аргументы прокси
		proxyArgs := getProxyArgs()
		
		// Команда yt-dlp для скачивания видео + аудио (поддержка до 2GB)
		args := []string{
			"--format", formatID + "+bestaudio/best", // Скачиваем видео + лучшее аудио
			"--output", filepath.Join(s.downloadDir, "%(id)s_" + formatID + ".%(ext)s"),
			"--no-playlist",
			"--no-check-certificates",
			"--max-filesize", "2G",    // Максимальный размер файла 2GB
			"--socket-timeout", "60",  // Увеличенный таймаут для больших файлов
			"--retries", "5",          // Больше попыток для больших файлов
			"--force-overwrites",      // Принудительно перезаписываем существующие файлы
			"--merge-output-format", "mp4", // Объединяем в MP4 с аудио
		}
		
		// Если это аудиоформат, принудительно конвертируем в MP3
		// Проверяем по ID формата - если содержит "drc", "audio", "webm" или другие аудио ID, это аудио
		if strings.Contains(formatID, "drc") || strings.Contains(formatID, "audio") || strings.Contains(formatID, "bestaudio") || strings.Contains(formatID, "webm") {
			args = append(args, "--extract-audio", "--audio-format", "mp3", "--audio-quality", "0")
			log.Printf("🎵 Обнаружен аудиоформат %s, принудительно конвертирую в MP3", formatID)
		}
		
		// Дополнительная проверка: если формат может дать webm файл, принудительно конвертируем в MP4
		// Это нужно для случаев когда видео скачивается в webm формате
		if strings.Contains(formatID, "webm") || strings.Contains(formatID, "251") || strings.Contains(formatID, "250") {
			args = append(args, "--recode-video", "mp4")
			log.Printf("🎬 Обнаружен WebM формат %s, принудительно конвертирую в MP4", formatID)
		}
		
		// Добавляем аргументы прокси
		args = append(args, proxyArgs...)
		args = append(args, videoURL)
		
		// Добавляем timeout для команды
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		
		cmd := exec.CommandContext(ctx, getYtDlpPath(), args...)

		log.Printf("🚀 Выполняю команду: %s", strings.Join(cmd.Args, " "))

		// Запускаем команду
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("❌ Ошибка yt-dlp: %s", string(output))
			return fmt.Errorf("ошибка yt-dlp: %v", err)
		}

		log.Printf("✅ yt-dlp выполнен успешно: %s", string(output))

		// Ищем скачанный файл для конкретного видео
		foundFile, findErr := s.findDownloadedFile(videoURL, formatID)
		if findErr != nil {
			return findErr
		}
		
		videoFile = foundFile
		return nil
	}, 2, 5*time.Second) // 2 попытки с базовой задержкой 5 секунд
	
	if err != nil {
		lastErr = err
		log.Printf("💥 Не удалось скачать видео после всех попыток: %v", err)
		return "", lastErr
	}

	return videoFile, nil
}

// findDownloadedFileOld ищет скачанный видео файл для конкретного URL (старая версия)
func (s *YouTubeService) findDownloadedFileOld(videoURL string) (string, error) {
	// Извлекаем ID видео из URL
	videoID := extractVideoID(videoURL)
	if videoID == "" {
		return "", fmt.Errorf("не удалось извлечь ID видео из URL: %s", videoURL)
	}

	files, err := os.ReadDir(s.downloadDir)
	if err != nil {
		return "", fmt.Errorf("не удалось прочитать папку загрузок: %v", err)
	}

	// Ищем файл с конкретным ID видео
	var videoFile string
	for _, file := range files {
		if !file.IsDir() && !strings.HasSuffix(file.Name(), ".webp") {
			// Проверяем, что файл содержит ID видео
			if strings.Contains(file.Name(), videoID) {
				videoFile = filepath.Join(s.downloadDir, file.Name())
				log.Printf("🎯 Найден файл для видео %s: %s", videoID, file.Name())
				break
			}
		}
	}

	if videoFile == "" {
		return "", fmt.Errorf("не найден скачанный видео файл для видео %s", videoID)
	}

	return videoFile, nil
}

// cleanVideoFiles очищает только файлы для конкретного видео и формата
func (s *YouTubeService) cleanVideoFiles(videoURL, formatID string) error {
	// Извлекаем ID видео из URL
	videoID := extractVideoID(videoURL)
	if videoID == "" {
		log.Printf("⚠️ Не удалось извлечь ID видео из URL: %s", videoURL)
		return nil
	}

	files, err := os.ReadDir(s.downloadDir)
	if err != nil {
		return fmt.Errorf("не удалось прочитать папку загрузок: %v", err)
	}

	// Удаляем только файлы с этим ID видео и форматом
	deletedCount := 0
	expectedPattern := videoID + "_" + formatID
	for _, file := range files {
		if !file.IsDir() && strings.Contains(file.Name(), expectedPattern) {
			filePath := filepath.Join(s.downloadDir, file.Name())
			if err := os.Remove(filePath); err != nil {
				log.Printf("⚠️ Не удалось удалить файл %s: %v", filePath, err)
			} else {
				log.Printf("🗑️ Удален файл для видео %s (формат %s): %s", videoID, formatID, filePath)
				deletedCount++
			}
		}
	}

	if deletedCount > 0 {
		log.Printf("🧹 Удалено %d файлов для видео %s", deletedCount, videoID)
	} else {
		log.Printf("ℹ️ Файлы для видео %s не найдены", videoID)
	}
	return nil
}

// extractVideoID извлекает ID видео из YouTube URL
func extractVideoID(url string) string {
	// Поддерживаем разные форматы YouTube URL
	patterns := []string{
		`youtube\.com/watch\?v=([a-zA-Z0-9_-]+)`,
		`youtu\.be/([a-zA-Z0-9_-]+)`,
		`youtube\.com/embed/([a-zA-Z0-9_-]+)`,
		`youtube\.com/shorts/([a-zA-Z0-9_-]+)`, // Добавляем поддержку YouTube Shorts
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(url)
		if len(matches) > 1 {
			return matches[1]
		}
	}

	return ""
}

// findDownloadedFile ищет скачанный видео файл для конкретного URL и формата
func (s *YouTubeService) findDownloadedFile(videoURL, formatID string) (string, error) {
	// Извлекаем ID видео из URL
	videoID := extractVideoID(videoURL)
	if videoID == "" {
		return "", fmt.Errorf("не удалось извлечь ID видео из URL: %s", videoURL)
	}

	files, err := os.ReadDir(s.downloadDir)
	if err != nil {
		return "", fmt.Errorf("не удалось прочитать папку загрузок: %v", err)
	}

	// Ищем файл с конкретным ID видео и форматом
	var videoFile string
	expectedPattern := videoID + "_" + formatID
	for _, file := range files {
		if !file.IsDir() && !strings.HasSuffix(file.Name(), ".webp") {
			// Проверяем, что файл содержит ID видео и formatID
			if strings.Contains(file.Name(), expectedPattern) {
				videoFile = filepath.Join(s.downloadDir, file.Name())
				log.Printf("🎯 Найден файл для видео %s (формат %s): %s", videoID, formatID, file.Name())
				break
			}
		}
	}

	if videoFile == "" {
		return "", fmt.Errorf("не найден скачанный видео файл для видео %s", videoID)
	}

	return videoFile, nil
}

// DownloadVideoFast быстро скачивает видео без анализа форматов
func (s *YouTubeService) DownloadVideoFast(url string) (string, error) {
	// Создаем папку для загрузок если не существует
	if err := os.MkdirAll(s.downloadDir, 0755); err != nil {
		return "", fmt.Errorf("не удалось создать папку для загрузок: %v", err)
	}

	log.Printf("⚡ Быстрое скачивание видео: %s", url)

	// Пробуем разные стратегии скачивания
	strategies := []struct {
		name string
		args []string
	}{
		{
			name: "Стандартное скачивание (до 2GB)",
			args: []string{
				"--format", "best[ext=mp4]/best",
				"--output", filepath.Join(s.downloadDir, "%(id)s.%(ext)s"),
				"--no-playlist",
				"--no-check-certificates",
				"--no-warnings",
				"--quiet",
				"--max-filesize", "2G",
				"--socket-timeout", "60",
				"--retries", "5",
			},
		},
		{
			name: "Скачивание с обходом ограничений (до 2GB)",
			args: []string{
				"--format", "best",
				"--output", filepath.Join(s.downloadDir, "%(id)s.%(ext)s"),
				"--no-playlist",
				"--no-check-certificates",
				"--no-warnings",
				"--quiet",
				"--extractor-args", "youtube:player_client=android",
				"--force-generic-extractor",
				"--max-filesize", "2G",
				"--socket-timeout", "60",
				"--retries", "5",
			},
		},
		{
			name: "Скачивание с прокси (до 2GB)",
			args: []string{
				"--format", "best",
				"--output", filepath.Join(s.downloadDir, "%(id)s.%(ext)s"),
				"--no-playlist",
				"--no-check-certificates",
				"--no-warnings",
				"--quiet",
				"--extractor-args", "youtube:player_client=android",
				"--max-filesize", "2G",
				"--socket-timeout", "60",
				"--retries", "5",
			},
		},
	}

	for i, strategy := range strategies {
		log.Printf("🔄 Попытка %d: %s", i+1, strategy.name)
		
		// Получаем аргументы прокси
		proxyArgs := getProxyArgs()
		
		// Добавляем аргументы прокси к стратегии
		args := append(strategy.args, proxyArgs...)
		args = append(args, url)
		
		// Добавляем timeout для команды
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		
		cmd := exec.CommandContext(ctx, getYtDlpPath(), args...)
		
		log.Printf("🚀 Выполняю команду: %s", strings.Join(cmd.Args, " "))
		
		output, err := cmd.CombinedOutput()
		if err == nil {
			log.Printf("✅ %s выполнен успешно: %s", strategy.name, string(output))
			
			// Ищем скачанный файл для конкретного видео (без формата для старого метода)
			videoFile, err := s.findDownloadedFileOld(url)
			if err != nil {
				continue // Пробуем следующую стратегию
			}
			
			return videoFile, nil
		}
		
		log.Printf("❌ %s не удался: %s", strategy.name, string(output))
	}

	return "", fmt.Errorf("все стратегии скачивания не удались")
}

// CheckYtDlp проверяет наличие yt-dlp в системе
func (s *YouTubeService) CheckYtDlp() error {
	// Сначала проверяем новый путь
	if _, err := exec.LookPath("/usr/local/bin/yt-dlp"); err == nil {
		log.Printf("✅ yt-dlp найден по пути /usr/local/bin/yt-dlp")
		return nil
	}
	
	// Если не найден, проверяем старый путь
	if _, err := exec.LookPath("yt-dlp"); err == nil {
		log.Printf("✅ yt-dlp найден по пути yt-dlp")
		return nil
	}
	
	return fmt.Errorf("yt-dlp не найден в системе. Проверьте установку")
}

// CheckNetwork проверяет сетевое подключение к YouTube
func (s *YouTubeService) CheckNetwork() error {
	log.Printf("🌐 Проверяю сетевое подключение к YouTube...")
	
	// Формируем команду curl с прокси
	args := []string{"-s", "--connect-timeout", "10", "--max-time", "30"}
	
	// Добавляем прокси если доступен
	proxyArgs := getProxyArgs()
	if len(proxyArgs) > 0 {
		// Извлекаем только --proxy аргумент из getProxyArgs
		for i, arg := range proxyArgs {
			if arg == "--proxy" && i+1 < len(proxyArgs) {
				args = append(args, "--proxy", proxyArgs[i+1])
				log.Printf("🌐 Проверка сети через прокси: %s", proxyArgs[i+1])
				break
			}
		}
	}
	
	args = append(args, "https://www.youtube.com")
	
	// Проверяем доступность YouTube
	cmd := exec.Command("curl", args...)
	
	if err := cmd.Run(); err != nil {
		log.Printf("⚠️ YouTube недоступен через curl: %v", err)
		return fmt.Errorf("проблемы с сетевым подключением к YouTube")
	}
	
	log.Printf("✅ Сетевое подключение к YouTube работает")
	return nil
}

// isBetterAudioQuality проверяет, является ли новый аудио формат лучше существующего
func (s *YouTubeService) isBetterAudioQuality(new, existing VideoFormat) bool {
	// Для аудио: больший размер обычно означает лучшее качество
	if new.FileSize != "" && existing.FileSize != "" {
		newSize := s.parseFileSize(new.FileSize)
		existingSize := s.parseFileSize(existing.FileSize)
		return newSize > existingSize
	}
	
	// Если не можем сравнить - считаем новый лучше
	return true
}

// isBetterQuality проверяет, является ли новый формат лучше существующего
func (s *YouTubeService) isBetterQuality(new, existing VideoFormat) bool {
	// Если у нового формата есть аудио, а у существующего нет - новый лучше
	if new.HasAudio && !existing.HasAudio {
		return true
	}
	
	// Если у обоих есть аудио или у обоих нет - сравниваем по размеру
	// Больший размер обычно означает лучшее качество
	if new.FileSize != "" && existing.FileSize != "" {
		newSize := s.parseFileSize(new.FileSize)
		existingSize := s.parseFileSize(existing.FileSize)
		return newSize > existingSize
	}
	
	// Если не можем сравнить - считаем новый лучше
	return true
}

// parseFileSize парсит размер файла в байты для сравнения
func (s *YouTubeService) parseFileSize(fileSize string) int64 {
	fileSize = strings.TrimSpace(fileSize)
	fileSize = strings.TrimPrefix(fileSize, "≈")
	fileSize = strings.TrimPrefix(fileSize, "~")
	
	var multiplier int64 = 1
	if strings.Contains(fileSize, "GiB") {
		multiplier = 1024 * 1024 * 1024
		fileSize = strings.Replace(fileSize, "GiB", "", 1)
	} else if strings.Contains(fileSize, "MiB") {
		multiplier = 1024 * 1024
		fileSize = strings.Replace(fileSize, "MiB", "", 1)
	} else if strings.Contains(fileSize, "KiB") {
		multiplier = 1024
		fileSize = strings.Replace(fileSize, "KiB", "", 1)
	} else if strings.Contains(fileSize, "B") {
		multiplier = 1
		fileSize = strings.Replace(fileSize, "B", "", 1)
	}
	
	if size, err := strconv.ParseFloat(fileSize, 64); err == nil {
		return int64(size * float64(multiplier))
	}
	
	return 0
}

// GetVideoMetadata получает метаданные видео (название, автор, длительность, просмотры)
func (s *YouTubeService) GetVideoMetadata(url string) (*VideoMetadata, error) {
	log.Printf("📊 Получение метаданных для: %s", url)
	
	// Получаем аргументы прокси
	proxyArgs := getProxyArgs()
	
	// Команда yt-dlp для получения метаданных
	args := []string{
		"--dump-json",           // Получаем JSON с метаданными
		"--no-playlist",         // Только одно видео
		"--no-check-certificates",
		"--no-warnings",
		"--quiet",
	}
	
	// Добавляем аргументы прокси
	args = append(args, proxyArgs...)
	args = append(args, url)
	
	// Добавляем timeout для команды метаданных
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, getYtDlpPath(), args...)
	
	log.Printf("🚀 Выполняю команду для метаданных: %s", strings.Join(cmd.Args, " "))
	
	// Запускаем команду
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("❌ Ошибка получения метаданных: %s", string(output))
		return nil, fmt.Errorf("ошибка получения метаданных: %v", err)
	}
	
	// Парсим JSON ответ
	metadata, err := s.parseVideoMetadata(string(output))
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга метаданных: %v", err)
	}
	
	// Устанавливаем оригинальный URL
	metadata.OriginalURL = url
	
	log.Printf("✅ Метаданные получены: %s - %s", metadata.Title, metadata.Author)
	log.Printf("🖼️ Миниатюра: %s", metadata.Thumbnail)
	log.Printf("⏱️ Длительность: %s", metadata.Duration)
	log.Printf("👁️ Просмотры: %s", metadata.Views)
	log.Printf("🔗 Оригинал: %s", metadata.OriginalURL)
	return metadata, nil
}

// parseVideoMetadata парсит JSON ответ yt-dlp и извлекает метаданные
func (s *YouTubeService) parseVideoMetadata(jsonOutput string) (*VideoMetadata, error) {
	// Парсим JSON
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonOutput), &data); err != nil {
		return nil, fmt.Errorf("ошибка парсинга JSON: %v", err)
	}
	
	// Логируем ключи JSON для отладки
	log.Printf("🔍 Ключи в JSON ответе: %v", getKeys(data))
	
	metadata := &VideoMetadata{}
	
	// Извлекаем название
	if title, ok := data["title"].(string); ok {
		metadata.Title = title
	}
	
	// Извлекаем автора
	if uploader, ok := data["uploader"].(string); ok {
		metadata.Author = uploader
	}
	
	// Извлекаем длительность
	if duration, ok := data["duration"].(float64); ok {
		metadata.Duration = s.formatDuration(int(duration))
	}
	
	// Извлекаем количество просмотров
	if viewCount, ok := data["view_count"].(float64); ok {
		metadata.Views = s.formatViews(int64(viewCount))
	}
	
	// Извлекаем описание
	if description, ok := data["description"].(string); ok {
		// Ограничиваем описание до 200 символов
		if len(description) > 200 {
			metadata.Description = description[:200] + "..."
		} else {
			metadata.Description = description
		}
	}
	
	// Извлекаем миниатюру (берем лучшую по качеству)
	if thumbnails, ok := data["thumbnails"].([]interface{}); ok && len(thumbnails) > 0 {
		// Ищем миниатюру с максимальным разрешением
		var bestThumbnail string
		var maxWidth int
		
		for _, thumb := range thumbnails {
			if thumbMap, ok := thumb.(map[string]interface{}); ok {
				if url, ok := thumbMap["url"].(string); ok {
					// Если есть информация о ширине - используем её
					if width, ok := thumbMap["width"].(float64); ok {
						if int(width) > maxWidth {
							maxWidth = int(width)
							bestThumbnail = url
						}
					} else {
						// Если нет информации о ширине - берем первую
						if bestThumbnail == "" {
							bestThumbnail = url
						}
					}
				}
			}
		}
		
		if bestThumbnail != "" {
			metadata.Thumbnail = bestThumbnail
			log.Printf("🖼️ Выбрана миниатюра: %s (ширина: %dpx)", bestThumbnail, maxWidth)
		}
	} else {
		// Альтернативный способ - используем thumbnail из корня JSON
		if thumbnail, ok := data["thumbnail"].(string); ok && thumbnail != "" {
			metadata.Thumbnail = thumbnail
			log.Printf("🖼️ Использована основная миниатюра: %s", thumbnail)
		} else {
			log.Printf("⚠️ Миниатюра не найдена в JSON ответе")
		}
	}
	
	// Извлекаем дату загрузки
	if uploadDate, ok := data["upload_date"].(string); ok {
		metadata.UploadDate = s.formatUploadDate(uploadDate)
	}
	
	// Извлекаем оригинальный URL
	if webpageURL, ok := data["webpage_url"].(string); ok {
		metadata.OriginalURL = webpageURL
	}
	
	return metadata, nil
}

// formatDuration форматирует длительность в читаемый вид
func (s *YouTubeService) formatDuration(seconds int) string {
	if seconds < 60 {
		return fmt.Sprintf("%d сек", seconds)
	} else if seconds < 3600 {
		minutes := seconds / 60
		remainingSeconds := seconds % 60
		if remainingSeconds == 0 {
			return fmt.Sprintf("%d мин", minutes)
		}
		return fmt.Sprintf("%d мин %d сек", minutes, remainingSeconds)
	} else {
		hours := seconds / 3600
		minutes := (seconds % 3600) / 60
		if minutes == 0 {
			return fmt.Sprintf("%d ч", hours)
		}
		return fmt.Sprintf("%d ч %d мин", hours, minutes)
	}
}

// formatViews форматирует количество просмотров
func (s *YouTubeService) formatViews(views int64) string {
	if views < 1000 {
		return fmt.Sprintf("%d", views)
	} else if views < 1000000 {
		return fmt.Sprintf("%.1fK", float64(views)/1000)
	} else if views < 1000000000 {
		return fmt.Sprintf("%.1fM", float64(views)/1000000)
	} else {
		return fmt.Sprintf("%.1fB", float64(views)/1000000000)
	}
}

// formatUploadDate форматирует дату загрузки
func (s *YouTubeService) formatUploadDate(uploadDate string) string {
	// Формат: YYYYMMDD
	if len(uploadDate) >= 8 {
		year := uploadDate[:4]
		month := uploadDate[4:6]
		day := uploadDate[6:8]
		return fmt.Sprintf("%s.%s.%s", day, month, year)
	}
	return uploadDate
}

// getKeys возвращает ключи из map для отладки
func getKeys(data map[string]interface{}) []string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	return keys
}
