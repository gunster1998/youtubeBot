package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"youtubeBot/config"
	"youtubeBot/services"
)

// AsyncLocalBot представляет бота с асинхронной обработкой
type AsyncLocalBot struct {
	Token          string
	APIURL         string
	Client         *http.Client
	Username       string
	FirstName      string
	// Кэш для хранения форматов по чатам
	formatCache    map[int64][]services.VideoFormat
	formatCacheMux sync.RWMutex
	// Кэш для хранения URL видео по чатам
	videoURLCache  map[int64]string
	videoURLCacheMux sync.RWMutex
	// Сервис для работы с YouTube
	youtubeService *services.YouTubeService
	// Сервис для кэширования популярных видео
	cacheService   *services.CacheService
	// Очередь загрузок
	downloadQueue  *services.DownloadQueue
	// Активные задачи пользователей
	userJobs       map[int64]string // chatID -> jobID
	userJobsMux    sync.RWMutex
}

// NewAsyncLocalBot создает новый экземпляр AsyncLocalBot
func NewAsyncLocalBot(token, apiURL string, timeout time.Duration, youtubeService *services.YouTubeService, cacheService *services.CacheService, downloadQueue *services.DownloadQueue) *AsyncLocalBot {
	return &AsyncLocalBot{
		Token:         token,
		APIURL:        apiURL,
		Client: &http.Client{
			Timeout: timeout,
		},
		formatCache:   make(map[int64][]services.VideoFormat),
		videoURLCache: make(map[int64]string),
		youtubeService: youtubeService,
		cacheService:  cacheService,
		downloadQueue: downloadQueue,
		userJobs:      make(map[int64]string),
	}
}

// GetMe получает информацию о боте
func (b *AsyncLocalBot) GetMe() error {
	resp, err := b.Client.Get(fmt.Sprintf("%s/bot%s/getMe", b.APIURL, b.Token))
	if err != nil {
		return fmt.Errorf("ошибка запроса getMe: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("неуспешный статус getMe: %d", resp.StatusCode)
	}

	var result struct {
		OK     bool `json:"ok"`
		Result struct {
			Username  string `json:"username"`
			FirstName string `json:"first_name"`
		} `json:"result"`
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("ошибка чтения ответа: %v", err)
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("ошибка парсинга JSON: %v", err)
	}

	if !result.OK {
		return fmt.Errorf("API вернул ошибку")
	}

	b.Username = result.Result.Username
	b.FirstName = result.Result.FirstName
	return nil
}

// SendMessage отправляет сообщение
func (b *AsyncLocalBot) SendMessage(chatID int64, text string) error {
	message := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("ошибка маршалинга сообщения: %v", err)
	}

	resp, err := b.Client.Post(
		fmt.Sprintf("%s/bot%s/sendMessage", b.APIURL, b.Token),
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return fmt.Errorf("ошибка отправки сообщения: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("неуспешный статус sendMessage: %d", resp.StatusCode)
	}

	return nil
}

// SendVideo отправляет видео файл
func (b *AsyncLocalBot) SendVideo(chatID int64, videoPath, caption string) error {
	file, err := os.Open(videoPath)
	if err != nil {
		return fmt.Errorf("ошибка открытия файла: %v", err)
	}
	defer file.Close()

	// Создаем multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Добавляем chat_id
	writer.WriteField("chat_id", fmt.Sprintf("%d", chatID))
	
	// Добавляем caption если есть
	if caption != "" {
		writer.WriteField("caption", caption)
	}

	// Добавляем файл
	part, err := writer.CreateFormFile("video", filepath.Base(videoPath))
	if err != nil {
		return fmt.Errorf("ошибка создания form file: %v", err)
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return fmt.Errorf("ошибка копирования файла: %v", err)
	}

	writer.Close()

	// Отправляем запрос
	resp, err := b.Client.Post(
		fmt.Sprintf("%s/bot%s/sendVideo", b.APIURL, b.Token),
		writer.FormDataContentType(),
		&buf,
	)
	if err != nil {
		return fmt.Errorf("ошибка отправки видео: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("неуспешный статус sendVideo: %d, ответ: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetUpdates получает обновления от Telegram
func (b *AsyncLocalBot) GetUpdates(offset, limit, timeout int) ([]Update, error) {
	resp, err := b.Client.Get(fmt.Sprintf("%s/bot%s/getUpdates?offset=%d&limit=%d&timeout=%d", 
		b.APIURL, b.Token, offset, limit, timeout))
	if err != nil {
		return nil, fmt.Errorf("ошибка запроса getUpdates: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("неуспешный статус getUpdates: %d", resp.StatusCode)
	}

	var result struct {
		OK     bool     `json:"ok"`
		Result []Update `json:"result"`
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %v", err)
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("ошибка парсинга JSON: %v", err)
	}

	if !result.OK {
		return nil, fmt.Errorf("API вернул ошибку")
	}

	return result.Result, nil
}

// handleYouTubeLink асинхронно обрабатывает YouTube ссылку
func (b *AsyncLocalBot) handleYouTubeLink(chatID int64, videoURL string) {
	// Проверяем, есть ли уже активная задача для этого пользователя
	b.userJobsMux.Lock()
	if existingJobID, exists := b.userJobs[chatID]; exists {
		// Отменяем предыдущую задачу
		if err := b.downloadQueue.CancelJob(existingJobID); err != nil {
			log.Printf("⚠️ Не удалось отменить предыдущую задачу %s: %v", existingJobID, err)
		}
		delete(b.userJobs, chatID)
	}
	b.userJobsMux.Unlock()

	// Очищаем кэш для этого чата
	b.formatCacheMux.Lock()
	delete(b.formatCache, chatID)
	b.formatCacheMux.Unlock()
	
	b.videoURLCacheMux.Lock()
	delete(b.videoURLCache, chatID)
	b.videoURLCacheMux.Unlock()

	log.Printf("🔍 Анализирую YouTube ссылку: %s", videoURL)
	b.SendMessage(chatID, "🔍 Анализирую доступные форматы видео...")

	// Запускаем анализ форматов в отдельной горутине
	go func() {
		// Получаем список форматов
		formats, err := b.youtubeService.GetVideoFormats(videoURL)
		if err != nil {
			log.Printf("❌ Ошибка GetVideoFormats: %v", err)
			b.SendMessage(chatID, fmt.Sprintf("❌ Ошибка получения форматов: %v", err))
			return
		}

		log.Printf("📊 Получено форматов: %d", len(formats))

		if len(formats) == 0 {
			log.Printf("⚠️ Форматы не найдены")
			b.SendMessage(chatID, "❌ Не найдено доступных форматов для скачивания.")
			return
		}

		// Сохраняем форматы и URL в кэше для этого чата
		b.formatCacheMux.Lock()
		b.formatCache[chatID] = formats
		b.formatCacheMux.Unlock()
		
		b.videoURLCacheMux.Lock()
		b.videoURLCache[chatID] = videoURL
		b.videoURLCacheMux.Unlock()

		log.Printf("💾 Сохранил в кэш: %d форматов и URL: %s для чата %d", len(formats), videoURL, chatID)

		// Разделяем форматы на аудио и видео
		var audioFormats []services.VideoFormat
		var videoFormats []services.VideoFormat

		// Группируем видео форматы по разрешению
		resolutionGroups := make(map[string][]services.VideoFormat)

		for _, format := range formats {
			if format.Extension == "audio" {
				audioFormats = append(audioFormats, format)
			} else {
				// Группируем по разрешению
				resolutionGroups[format.Resolution] = append(resolutionGroups[format.Resolution], format)
			}
		}

		// Для каждого разрешения выбираем ТОЛЬКО форматы С АУДИО
		for _, formats := range resolutionGroups {
			if len(formats) == 0 {
				continue
			}
			
			// Ищем формат с аудио для этого разрешения
			var audioFormat *services.VideoFormat
			for _, f := range formats {
				if f.HasAudio {
					audioFormat = &f
					break
				}
			}
			
			// Добавляем ТОЛЬКО если есть аудио
			if audioFormat != nil {
				videoFormats = append(videoFormats, *audioFormat)
			}
		}

		log.Printf("📊 Найдено %d аудио и %d видео форматов", len(audioFormats), len(videoFormats))

		// Проверяем, есть ли видео форматы с аудио
		if len(videoFormats) == 0 {
			log.Printf("⚠️ НЕ НАЙДЕНО видео форматов с аудио!")
			b.SendMessage(chatID, "❌ Не найдено видео форматов с аудио. Попробуйте другое видео.")
			return
		}

		// Сортируем видео форматы по разрешению (от меньшего к большему)
		sortVideoFormatsByResolution(videoFormats)

		// Отправляем подменю выбора типа
		if err := b.SendFormatTypeMenu(chatID, len(audioFormats), len(videoFormats)); err != nil {
			log.Printf("❌ Ошибка отправки меню выбора типа: %v", err)
			b.SendMessage(chatID, "❌ Ошибка создания меню выбора")
		}
	}()
}

// handleFormatSelection обрабатывает выбор формата пользователем
func (b *AsyncLocalBot) handleFormatSelection(chatID int64, formatID string) {
	// Получаем URL видео из кэша
	b.videoURLCacheMux.RLock()
	videoURL := b.videoURLCache[chatID]
	b.videoURLCacheMux.RUnlock()

	if videoURL == "" {
		log.Printf("❌ URL видео не найден в кэше для чата %d", chatID)
		b.SendMessage(chatID, "❌ Ошибка: URL видео не найден. Отправьте ссылку заново.")
		return
	}

	// Добавляем задачу в очередь
	jobID, err := b.downloadQueue.AddJob(chatID, chatID, videoURL, formatID, 5) // Приоритет 5 (средний)
	if err != nil {
		log.Printf("❌ Ошибка добавления задачи в очередь: %v", err)
		b.SendMessage(chatID, "❌ Ошибка: не удалось добавить задачу в очередь. Попробуйте позже.")
		return
	}

	// Сохраняем связь пользователь -> задача
	b.userJobsMux.Lock()
	b.userJobs[chatID] = jobID
	b.userJobsMux.Unlock()

	log.Printf("📝 Задача добавлена в очередь: %s для пользователя %d", jobID, chatID)
	b.SendMessage(chatID, "⏳ Задача добавлена в очередь загрузок. Ожидайте...")

	// Запускаем мониторинг задачи
	go b.monitorJob(chatID, jobID)
	
	// Дополнительно запускаем быструю проверку для кэшированных видео
	go b.quickCheckForCachedVideo(chatID, jobID, videoURL, formatID)
}

// quickCheckForCachedVideo быстро проверяет кэшированные видео
func (b *AsyncLocalBot) quickCheckForCachedVideo(chatID int64, jobID, videoURL, formatID string) {
	// Ждем немного, чтобы дать время очереди обработать задачу
	time.Sleep(1 * time.Second)
	
	// Проверяем, есть ли задача в активных
	job, exists := b.downloadQueue.GetJobStatus(jobID)
	if !exists {
		// Задача уже завершена - возможно это кэшированное видео
		log.Printf("🔍 Быстрая проверка: задача %s уже завершена, проверяю кэш", jobID)
		
		// Проверяем кэш напрямую
		videoID := extractVideoID(videoURL)
		if videoID != "" {
			if isCached, cachedVideo, err := b.cacheService.IsVideoCached(videoID, formatID); err == nil && isCached {
				log.Printf("⚡ Быстрая проверка: найдено кэшированное видео для задачи %s", jobID)
				
				// Отправляем кэшированное видео
				b.SendMessage(chatID, "✅ Видео найдено в кэше! Отправляю файл...")
				if err := b.SendVideo(chatID, cachedVideo.FilePath, fmt.Sprintf("Видео в формате %s", formatID)); err != nil {
					log.Printf("❌ Ошибка отправки кэшированного видео: %v", err)
					b.SendMessage(chatID, "❌ Ошибка отправки файла")
				} else {
					b.SendMessage(chatID, "🎉 Видео успешно отправлено!")
				}
				
				// Удаляем задачу из активных пользователя
				b.userJobsMux.Lock()
				delete(b.userJobs, chatID)
				b.userJobsMux.Unlock()
				return
			}
		}
	} else if job.Status == services.JobStatusCompleted {
		// Задача уже завершена
		log.Printf("✅ Быстрая проверка: задача %s уже завершена", jobID)
		b.SendMessage(chatID, "✅ Видео готово! Отправляю файл...")
		
		if err := b.SendVideo(chatID, job.Result, fmt.Sprintf("Видео в формате %s", job.FormatID)); err != nil {
			log.Printf("❌ Ошибка отправки видео: %v", err)
			b.SendMessage(chatID, "❌ Ошибка отправки файла")
		} else {
			b.SendMessage(chatID, "🎉 Видео успешно отправлено!")
		}
		
		// Удаляем задачу из активных
		b.userJobsMux.Lock()
		delete(b.userJobs, chatID)
		b.userJobsMux.Unlock()
	}
}

// monitorJob отслеживает выполнение задачи
func (b *AsyncLocalBot) monitorJob(chatID int64, jobID string) {
	ticker := time.NewTicker(2 * time.Second) // Уменьшаем интервал для быстрого отклика
	defer ticker.Stop()

	timeout := time.NewTimer(10 * time.Minute) // Таймаут 10 минут
	defer timeout.Stop()

	for {
		select {
		case <-ticker.C:
			job, exists := b.downloadQueue.GetJobStatus(jobID)
			if !exists {
				// Задача не найдена - возможно уже завершена (особенно для кэшированных видео)
				// Для кэшированных видео это нормально - они обрабатываются мгновенно
				log.Printf("⚠️ Задача %s не найдена в активных - возможно завершена", jobID)
				
				// Удаляем задачу из активных пользователя
				b.userJobsMux.Lock()
				delete(b.userJobs, chatID)
				b.userJobsMux.Unlock()
				return
			}

			switch job.Status {
			case services.JobStatusCompleted:
				log.Printf("✅ Задача %s завершена успешно", jobID)
				b.SendMessage(chatID, "✅ Видео готово! Отправляю файл...")
				
				// Отправляем файл
				if err := b.SendVideo(chatID, job.Result, fmt.Sprintf("Видео в формате %s", job.FormatID)); err != nil {
					log.Printf("❌ Ошибка отправки видео: %v", err)
					b.SendMessage(chatID, "❌ Ошибка отправки файла")
				} else {
					b.SendMessage(chatID, "🎉 Видео успешно отправлено!")
				}

				// Удаляем задачу из активных
				b.userJobsMux.Lock()
				delete(b.userJobs, chatID)
				b.userJobsMux.Unlock()
				return

			case services.JobStatusFailed:
				log.Printf("❌ Задача %s завершена с ошибкой: %v", jobID, job.Error)
				b.SendMessage(chatID, fmt.Sprintf("❌ Ошибка загрузки: %v", job.Error))

				// Удаляем задачу из активных
				b.userJobsMux.Lock()
				delete(b.userJobs, chatID)
				b.userJobsMux.Unlock()
				return

			case services.JobStatusCancelled:
				log.Printf("❌ Задача %s отменена", jobID)
				b.SendMessage(chatID, "❌ Задача отменена")

				// Удаляем задачу из активных
				b.userJobsMux.Lock()
				delete(b.userJobs, chatID)
				b.userJobsMux.Unlock()
				return

			case services.JobStatusProcessing:
				// Задача выполняется, продолжаем ждать
				continue

			case services.JobStatusPending:
				// Задача в очереди, продолжаем ждать
				continue
			}

		case <-timeout.C:
			log.Printf("⏰ Таймаут ожидания задачи %s", jobID)
			b.SendMessage(chatID, "⏰ Время ожидания истекло. Попробуйте позже.")

			// Удаляем задачу из активных
			b.userJobsMux.Lock()
			delete(b.userJobs, chatID)
			b.userJobsMux.Unlock()
			return
		}
	}
}

// SendFormatTypeMenu отправляет меню выбора типа формата (аудио/видео)
func (b *AsyncLocalBot) SendFormatTypeMenu(chatID int64, audioCount, videoCount int) error {
	log.Printf("🎯 Создаю меню выбора типа: аудио=%d, видео=%d", audioCount, videoCount)
	
	// Создаем inline keyboard для выбора типа
	var keyboard [][]map[string]interface{}
	
	// Кнопка для аудио форматов
	if audioCount > 0 {
		keyboard = append(keyboard, []map[string]interface{}{
			{
				"text":          "🎵 Аудио форматы",
				"callback_data": "type_audio",
			},
		})
	}
	
	// Кнопка для видео форматов
	if videoCount > 0 {
		keyboard = append(keyboard, []map[string]interface{}{
			{
				"text":          "🎥 Видео форматы",
				"callback_data": "type_video",
			},
		})
	}
	
	// Создаем сообщение с keyboard
	message := map[string]interface{}{
		"chat_id":      chatID,
		"text":         "💡 Выберите тип формата для скачивания:",
		"reply_markup": map[string]interface{}{"inline_keyboard": keyboard},
	}
	
	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("ошибка маршалинга keyboard: %v", err)
	}
	
	// Отправляем запрос
	resp, err := b.Client.Post(
		fmt.Sprintf("%s/bot%s/sendMessage", b.APIURL, b.Token),
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return fmt.Errorf("ошибка отправки keyboard: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("неуспешный статус отправки keyboard: %d, ответ: %s", resp.StatusCode, string(bodyBytes))
	}
	
	log.Printf("✅ Меню выбора типа отправлено успешно")
	return nil
}

// SendVideoFormatsOnly отправляет только видео форматы
func (b *AsyncLocalBot) SendVideoFormatsOnly(chatID int64, text string, formats []services.VideoFormat) error {
	log.Printf("🎥 Отправляю только видео форматы (%d штук)", len(formats))
	
	// Создаем inline keyboard только для видео форматов
	var keyboard [][]map[string]interface{}
	
	// Добавляем кнопки для каждого формата
	for _, format := range formats {
		icon := "🎥"
		
		buttonText := fmt.Sprintf("%s %s / %s", icon, format.Resolution, format.FileSize)
		if format.FileSize == "" {
			buttonText = fmt.Sprintf("%s %s / ~?", icon, format.Resolution)
		}
		
		// Создаем callback data для кнопки
		callbackData := fmt.Sprintf("format_%s_%s", format.ID, format.Resolution)
		
		keyboard = append(keyboard, []map[string]interface{}{
			{
				"text":          buttonText,
				"callback_data": callbackData,
			},
		})
	}
	
	// Создаем сообщение с keyboard
	message := map[string]interface{}{
		"chat_id":      chatID,
		"text":         text,
		"reply_markup": map[string]interface{}{"inline_keyboard": keyboard},
	}
	
	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("ошибка маршалинга keyboard: %v", err)
	}
	
	// Отправляем запрос
	resp, err := b.Client.Post(
		fmt.Sprintf("%s/bot%s/sendMessage", b.APIURL, b.Token),
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return fmt.Errorf("ошибка отправки keyboard: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("неуспешный статус отправки keyboard: %d, ответ: %s", resp.StatusCode, string(bodyBytes))
	}
	
	log.Printf("✅ Видео форматы отправлены успешно (%d кнопок)", len(keyboard))
	return nil
}

// SendAudioFormatsOnly отправляет только аудио форматы
func (b *AsyncLocalBot) SendAudioFormatsOnly(chatID int64, text string, formats []services.VideoFormat) error {
	log.Printf("🎵 Отправляю только аудио форматы (%d штук)", len(formats))
	
	// Создаем inline keyboard только для аудио форматов
	var keyboard [][]map[string]interface{}
	
	// Добавляем кнопки для каждого формата
	for _, format := range formats {
		icon := "🎵"
		
		buttonText := fmt.Sprintf("%s %s / %s", icon, format.Resolution, format.FileSize)
		if format.FileSize == "" {
			buttonText = fmt.Sprintf("%s %s / ~?", icon, format.Resolution)
		}
		
		// Создаем callback data для кнопки
		callbackData := fmt.Sprintf("format_%s_%s", format.ID, format.Resolution)
		
		keyboard = append(keyboard, []map[string]interface{}{
			{
				"text":          buttonText,
				"callback_data": callbackData,
			},
		})
	}
	
	// Создаем сообщение с keyboard
	message := map[string]interface{}{
		"chat_id":      chatID,
		"text":         text,
		"reply_markup": map[string]interface{}{"inline_keyboard": keyboard},
	}
	
	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("ошибка маршалинга keyboard: %v", err)
	}
	
	// Отправляем запрос
	resp, err := b.Client.Post(
		fmt.Sprintf("%s/bot%s/sendMessage", b.APIURL, b.Token),
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return fmt.Errorf("ошибка отправки keyboard: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("неуспешный статус отправки keyboard: %d, ответ: %s", resp.StatusCode, string(bodyBytes))
	}
	
	log.Printf("✅ Аудио форматы отправлены успешно (%d кнопок)", len(keyboard))
	return nil
}

// AnswerCallbackQuery отвечает на callback query
func (b *AsyncLocalBot) AnswerCallbackQuery(callbackID string) error {
	message := map[string]interface{}{
		"callback_query_id": callbackID,
	}
	
	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("ошибка маршалинга callback answer: %v", err)
	}
	
	// Отправляем запрос
	resp, err := b.Client.Post(
		fmt.Sprintf("%s/bot%s/answerCallbackQuery", b.APIURL, b.Token),
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return fmt.Errorf("ошибка ответа на callback: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("неуспешный статус callback answer: %d, ответ: %s", resp.StatusCode, string(bodyBytes))
	}
	
	return nil
}

// Update представляет обновление от Telegram
type Update struct {
	UpdateID int64   `json:"update_id"`
	Message  *Message `json:"message,omitempty"`
	CallbackQuery *CallbackQuery `json:"callback_query,omitempty"`
}

// CallbackQuery представляет callback от inline keyboard
type CallbackQuery struct {
	ID   string  `json:"id"`
	Data string  `json:"data"`
	Message *Message `json:"message"`
}

// Message представляет сообщение от Telegram
type Message struct {
	MessageID int64  `json:"message_id"`
	Text      string `json:"text"`
	Chat      Chat   `json:"chat"`
}

// Chat представляет чат в Telegram
type Chat struct {
	ID int64 `json:"id"`
}

func main() {
	// Загружаем конфигурацию
	cfg, err := config.Load("config.env")
	if err != nil {
		log.Fatalf("❌ Ошибка загрузки конфигурации: %v", err)
	}

	// Проверяем токен
	if cfg.TelegramToken == "" {
		log.Fatal("❌ TELEGRAM_BOT_TOKEN не установлен в config.env")
	}

	fmt.Printf("🚀 Запуск асинхронного бота с локальным сервером Telegram API: %s\n", cfg.TelegramAPI)

	// Проверяем yt-dlp
	youtubeService := services.NewYouTubeService(cfg.DownloadDir)
	if err := youtubeService.CheckYtDlp(); err != nil {
		log.Fatalf("❌ %v", err)
	}
	fmt.Println("✅ yt-dlp доступен")

	// Создаем сервис для кэширования (20 ГБ)
	cacheService, err := services.NewCacheService("../cache", 20)
	if err != nil {
		log.Fatalf("❌ Ошибка создания кэш-сервиса: %v", err)
	}
	defer cacheService.Close()
	
	// Создаем очередь загрузок с 3 воркерами
	downloadQueue := services.NewDownloadQueue(3, youtubeService, cacheService)
	downloadQueue.Start()
	defer downloadQueue.Stop()
	
	// Создаем асинхронного бота
	bot := NewAsyncLocalBot(cfg.TelegramToken, cfg.TelegramAPI, time.Duration(cfg.HTTPTimeout)*time.Second, youtubeService, cacheService, downloadQueue)

	// Проверяем подключение к локальному серверу Telegram API
	if err := bot.GetMe(); err != nil {
		log.Fatalf("❌ Не удалось подключиться к локальному серверу Telegram API: %v", err)
	}

	fmt.Printf("✅ Бот успешно подключен: @%s (%s)\n", bot.Username, bot.FirstName)
	fmt.Printf("🌐 Используется локальный сервер: %s\n", cfg.TelegramAPI)

	// Проверяем сетевое подключение
	if err := youtubeService.CheckNetwork(); err != nil {
		log.Printf("⚠️ %v", err)
		fmt.Println("⚠️ Проблемы с сетью - бот может работать нестабильно")
	} else {
		fmt.Println("✅ Сетевое подключение работает")
	}

	fmt.Println("🎬 Асинхронный бот готов к работе! Отправьте ссылку на YouTube видео.")

	// Обрабатываем сигналы для graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Основной цикл получения обновлений через getUpdates
	log.Printf("🔄 Запуск цикла getUpdates...")
	
	offset := int64(0)
	for {
		select {
		case <-sigChan:
			fmt.Printf("\n🛑 Получен сигнал завершения, завершаю работу...\n")
			return
		default:
			// Получаем обновления
			updates, err := bot.GetUpdates(int(offset), 100, 30)
			if err != nil {
				log.Printf("⚠️ Ошибка получения обновлений: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}

			// Обрабатываем обновления
			for _, update := range updates {
				if update.UpdateID >= offset {
					offset = update.UpdateID + 1
				}

				if update.Message != nil {
					message := update.Message
					log.Printf("📨 Получено сообщение: %s от чата %d", message.Text, message.Chat.ID)
					
					// Обрабатываем команды
					if message.Text == "/start" {
						bot.SendMessage(message.Chat.ID, "Привет! Отправьте ссылку на YouTube видео для скачивания.")
					} else if len(message.Text) > 10 && (strings.Contains(message.Text, "youtube.com") || strings.Contains(message.Text, "youtu.be")) {
						// YouTube ссылка - обрабатываем асинхронно
						bot.handleYouTubeLink(message.Chat.ID, message.Text)
					} else {
						bot.SendMessage(message.Chat.ID, "Отправьте ссылку на YouTube видео для скачивания.")
					}
				} else if update.CallbackQuery != nil {
					// Обрабатываем callback от inline keyboard
					callback := update.CallbackQuery
					log.Printf("🎯 Получен callback: %s", callback.Data)
					
					if callback.Data == "type_audio" {
						// Пользователь выбрал аудио форматы
						log.Printf("🎵 Пользователь выбрал аудио форматы")
						bot.AnswerCallbackQuery(callback.ID)
						
						// Показываем список аудио форматов
						bot.formatCacheMux.RLock()
						formats := bot.formatCache[callback.Message.Chat.ID]
						bot.formatCacheMux.RUnlock()
						
						var audioFormats []services.VideoFormat
						for _, format := range formats {
							if format.Extension == "audio" {
								audioFormats = append(audioFormats, format)
							}
						}
						
						if len(audioFormats) > 0 {
							bot.SendAudioFormatsOnly(callback.Message.Chat.ID, "🎵 Аудио форматы:", audioFormats)
						} else {
							bot.SendMessage(callback.Message.Chat.ID, "❌ Аудио форматы не найдены")
						}
						
					} else if callback.Data == "type_video" {
						// Пользователь выбрал видео форматы
						log.Printf("🎥 Пользователь выбрал видео форматы")
						bot.AnswerCallbackQuery(callback.ID)
						
						// Получаем форматы из кэша и применяем умную группировку
						bot.formatCacheMux.RLock()
						formats := bot.formatCache[callback.Message.Chat.ID]
						bot.formatCacheMux.RUnlock()
						
						// Группируем видео форматы по разрешению
						resolutionGroups := make(map[string][]services.VideoFormat)
						
						for _, format := range formats {
							if format.Extension != "audio" {
								resolutionGroups[format.Resolution] = append(resolutionGroups[format.Resolution], format)
							}
						}
						
						// Для каждого разрешения выбираем ЛУЧШИЙ формат
						var videoFormats []services.VideoFormat
						for _, formatList := range resolutionGroups {
							if len(formatList) == 0 {
								continue
							}
							
							// Сортируем форматы по размеру файла
							sort.Slice(formatList, func(i, j int) bool {
								sizeI := parseFileSize(formatList[i].FileSize)
								sizeJ := parseFileSize(formatList[j].FileSize)
								return sizeI < sizeJ
							})
							
							// Выбираем лучший формат для этого разрешения
							var bestFormat *services.VideoFormat
							
							// Сначала ищем формат с аудио
							for _, f := range formatList {
								if f.HasAudio {
									bestFormat = &f
									break
								}
							}
							
							// Если нет формата с аудио, берем самый маленький
							if bestFormat == nil {
								bestFormat = &formatList[0]
							}
							
							videoFormats = append(videoFormats, *bestFormat)
						}
						
						// Сортируем по разрешению
						sortVideoFormatsByResolution(videoFormats)
						
						if len(videoFormats) > 0 {
							bot.SendVideoFormatsOnly(callback.Message.Chat.ID, "🎥 Видео форматы:", videoFormats)
						} else {
							bot.SendMessage(callback.Message.Chat.ID, "❌ Не найдено видео форматов с аудио. Попробуйте другое видео.")
						}
						
					} else if strings.HasPrefix(callback.Data, "format_") {
						// Пользователь выбрал формат
						parts := strings.Split(callback.Data, "_")
						if len(parts) >= 2 {
							formatID := parts[1]
							log.Printf("📹 Пользователь выбрал формат: %s", formatID)
							bot.AnswerCallbackQuery(callback.ID)
							
							// Обрабатываем выбор формата асинхронно
							bot.handleFormatSelection(callback.Message.Chat.ID, formatID)
						}
					}
				}
			}

			// Небольшая пауза между запросами
			time.Sleep(1 * time.Second)
		}
	}
}

// parseFileSize парсит размер файла в байты
func parseFileSize(fileSize string) int64 {
	if fileSize == "" {
		return 0
	}
	
	fileSize = strings.TrimSpace(fileSize)
	
	var multiplier float64 = 1
	var sizeStr string
	
	if strings.HasSuffix(fileSize, "GiB") {
		multiplier = 1024 * 1024 * 1024
		sizeStr = strings.TrimSuffix(fileSize, "GiB")
	} else if strings.HasSuffix(fileSize, "MiB") {
		multiplier = 1024 * 1024
		sizeStr = strings.TrimSuffix(fileSize, "MiB")
	} else if strings.HasSuffix(fileSize, "KiB") {
		multiplier = 1024
		sizeStr = strings.TrimSuffix(fileSize, "KiB")
	} else if strings.HasSuffix(fileSize, "B") {
		multiplier = 1
		sizeStr = strings.TrimSuffix(fileSize, "B")
	} else {
		if size, err := strconv.ParseFloat(fileSize, 64); err == nil {
			return int64(size)
		}
		return 0
	}
	
	if size, err := strconv.ParseFloat(sizeStr, 64); err == nil {
		return int64(size * multiplier)
	}
	
	return 0
}

// sortVideoFormatsByResolution сортирует видео форматы по разрешению
func sortVideoFormatsByResolution(formats []services.VideoFormat) {
	sort.Slice(formats, func(i, j int) bool {
		resI := extractResolutionNumber(formats[i].Resolution)
		resJ := extractResolutionNumber(formats[j].Resolution)
		return resI < resJ
	})
}

// extractResolutionNumber извлекает числовое значение разрешения
func extractResolutionNumber(resolution string) int {
	re := regexp.MustCompile(`(\d+)`)
	matches := re.FindStringSubmatch(resolution)
	if len(matches) > 1 {
		if num, err := strconv.ParseFloat(matches[1], 64); err == nil {
			return int(num)
		}
	}
	return 0
}

// extractVideoID извлекает ID видео из YouTube URL
func extractVideoID(url string) string {
	// Поддерживаем разные форматы YouTube URL
	patterns := []string{
		`youtube\.com/watch\?v=([a-zA-Z0-9_-]+)`,
		`youtu\.be/([a-zA-Z0-9_-]+)`,
		`youtube\.com/embed/([a-zA-Z0-9_-]+)`,
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
