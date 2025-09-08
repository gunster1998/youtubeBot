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
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"youtubeBot/config"
	"youtubeBot/services"
)

// LocalBot представляет бота для работы с локальным сервером Telegram API
type LocalBot struct {
	Token    string
	APIURL   string
	Client   *http.Client
	Username string
	FirstName string
	// Кэш для хранения форматов по чатам
	formatCache map[int64][]services.VideoFormat
	// Кэш для хранения URL видео по чатам
	videoURLCache map[int64]string
	// Кэш для хранения информации о платформе по чатам
	platformCache map[int64]string
	// Сервис для работы с YouTube
	youtubeService *services.YouTubeService
	// Универсальный сервис для работы с разными платформами
	universalService *services.UniversalService
	// Сервис для кэширования популярных видео
	cacheService *services.CacheService
	// Защита от спама - время последнего запроса по чатам
	lastRequestTime map[int64]time.Time
	// Метрики производительности
	metrics *BotMetrics
	// ID администраторов
	adminIDs map[int64]bool
}

// BotMetrics содержит метрики производительности бота
type BotMetrics struct {
	StartTime        time.Time
	TotalRequests    int64
	SuccessfulRequests int64
	FailedRequests   int64
	TotalDownloads   int64
	TotalErrors      int64
	AverageResponseTime time.Duration
	LastActivity     time.Time
}

// NewLocalBot создает новый экземпляр LocalBot
func NewLocalBot(token, apiURL string, timeout time.Duration, youtubeService *services.YouTubeService, universalService *services.UniversalService, cacheService *services.CacheService) *LocalBot {
	// Создаем карту администраторов
	adminIDs := make(map[int64]bool)
	adminIDs[6717533619] = true  // Первый администратор
	adminIDs[234549643] = true   // Второй администратор
	
	return &LocalBot{
		Token:  token,
		APIURL: apiURL,
		Client: &http.Client{
			Timeout: timeout,
		},
		formatCache: make(map[int64][]services.VideoFormat),
		videoURLCache: make(map[int64]string),
		platformCache: make(map[int64]string),
		youtubeService: youtubeService,
		universalService: universalService,
		cacheService: cacheService,
		lastRequestTime: make(map[int64]time.Time),
		metrics: &BotMetrics{
			StartTime: time.Now(),
			LastActivity: time.Now(),
		},
		adminIDs: adminIDs,
	}
}

// GetMe получает информацию о боте
func (b *LocalBot) GetMe() error {
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
func (b *LocalBot) SendMessage(chatID int64, text string) error {
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

// ClearChatHistory очищает историю чата (удаляет сообщения бота)
func (b *LocalBot) ClearChatHistory(chatID int64) error {
	// Получаем последние сообщения бота
	updates, err := b.GetUpdates(0, 100, 0)
	if err != nil {
		return fmt.Errorf("ошибка получения обновлений: %v", err)
	}

	// Удаляем сообщения бота в этом чате
	for _, update := range updates {
		if update.Message != nil && update.Message.Chat.ID == chatID {
			// Удаляем сообщение бота
			deleteMessage := map[string]interface{}{
				"chat_id":    chatID,
				"message_id": update.Message.MessageID,
			}

			jsonData, err := json.Marshal(deleteMessage)
			if err != nil {
				continue
			}

			resp, err := b.Client.Post(
				fmt.Sprintf("%s/bot%s/deleteMessage", b.APIURL, b.Token),
				"application/json",
				bytes.NewBuffer(jsonData),
			)
			if err != nil {
				continue
			}
			resp.Body.Close()
		}
	}

	log.Printf("🧹 Очистил историю чата %d", chatID)
	return nil
}

// SendVideoPreview отправляет превью видео с метаданными и миниатюрой
func (b *LocalBot) SendVideoPreview(chatID int64, metadata *services.VideoMetadata) error {
	log.Printf("🎬 Начинаю отправку превью для чата %d", chatID)
	log.Printf("🎬 Метаданные: Title=%s, Author=%s, Thumbnail=%s", metadata.Title, metadata.Author, metadata.Thumbnail)
	
	// Создаем красивое превью
	previewText := fmt.Sprintf(`🎬 **%s**

👤 **Автор:** %s
⏱️ **Длительность:** %s
👁️ **Просмотры:** %s
📅 **Дата:** %s

📝 **Описание:**
%s

🔗 Выберите качество для скачивания:`, 
		metadata.Title,
		metadata.Author,
		metadata.Duration,
		metadata.Views,
		metadata.UploadDate,
		metadata.Description)
	
	// Если есть миниатюра - отправляем фото с подписью
	if metadata.Thumbnail != "" {
		log.Printf("🖼️ Отправляю превью с миниатюрой: %s", metadata.Thumbnail)
		log.Printf("🖼️ Текст превью: %s", previewText)
		err := b.SendPhoto(chatID, metadata.Thumbnail, previewText)
		if err != nil {
			log.Printf("❌ Ошибка SendPhoto: %v", err)
			return err
		}
		log.Printf("✅ SendPhoto выполнен успешно")
		return nil
	}
	
	// Если нет миниатюры - отправляем только текст
	log.Printf("⚠️ Миниатюра не найдена, отправляю только текст")
	err := b.SendMessage(chatID, previewText)
	if err != nil {
		log.Printf("❌ Ошибка SendMessage: %v", err)
		return err
	}
	log.Printf("✅ SendMessage выполнен успешно")
	return nil
}

// SendVideo отправляет видео файл
func (b *LocalBot) SendVideo(chatID int64, videoPath, caption string) error {
	log.Printf("🎬 Отправляю видео: chatID=%d, path=%s", chatID, videoPath)
	
	file, err := os.Open(videoPath)
	if err != nil {
		return fmt.Errorf("ошибка открытия файла: %v", err)
	}
	defer file.Close()

	// Получаем информацию о файле
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("ошибка получения информации о файле: %v", err)
	}

	// Создаем multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Добавляем chat_id
	writer.WriteField("chat_id", fmt.Sprintf("%d", chatID))
	
	// Добавляем caption с описанием бота
	botCaption := fmt.Sprintf("%s\n\n🤖 Скачано через @YouLoaderTube_bot\n🔗 https://t.me/YouLoaderTube_bot", caption)
	writer.WriteField("caption", botCaption)

	// Добавляем длительность (в секундах)
	// Пытаемся получить длительность из метаданных файла
	duration := b.getVideoDuration(videoPath)
	if duration > 0 {
		writer.WriteField("duration", fmt.Sprintf("%d", duration))
		log.Printf("⏱️ Установлена длительность: %d секунд", duration)
	}

	// Добавляем миниатюру если есть
	thumbnailPath := b.getVideoThumbnail(videoPath)
	if thumbnailPath != "" {
		// Добавляем миниатюру как файл
		thumbFile, err := os.Open(thumbnailPath)
		if err == nil {
			defer thumbFile.Close()
			thumbWriter, err := writer.CreateFormFile("thumbnail", filepath.Base(thumbnailPath))
			if err == nil {
				io.Copy(thumbWriter, thumbFile)
				log.Printf("🖼️ Добавлена миниатюра: %s", thumbnailPath)
			}
		}
		// Удаляем миниатюру после отправки
		defer func() {
			if err := os.Remove(thumbnailPath); err != nil {
				log.Printf("⚠️ Не удалось удалить миниатюру: %v", err)
			} else {
				log.Printf("🗑️ Миниатюра удалена: %s", thumbnailPath)
			}
		}()
	}

	// Добавляем размер файла
	writer.WriteField("file_size", fmt.Sprintf("%d", fileInfo.Size()))

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
		log.Printf("❌ Ошибка sendVideo: %d, ответ: %s", resp.StatusCode, string(body))
		return fmt.Errorf("неуспешный статус sendVideo: %d, ответ: %s", resp.StatusCode, string(body))
	}

	log.Printf("✅ Видео отправлено успешно с миниатюрой и длительностью")
	return nil
}

// getVideoDuration получает длительность видео в секундах
func (b *LocalBot) getVideoDuration(videoPath string) int {
	// Используем ffprobe для получения длительности
	cmd := exec.Command("ffprobe", "-v", "quiet", "-show_entries", "format=duration", "-of", "csv=p=0", videoPath)
	output, err := cmd.Output()
	if err != nil {
		log.Printf("⚠️ Не удалось получить длительность видео: %v", err)
		return 0
	}
	
	durationStr := strings.TrimSpace(string(output))
	duration, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		log.Printf("⚠️ Не удалось распарсить длительность: %v", err)
		return 0
	}
	
	return int(duration)
}

// getVideoThumbnail получает путь к миниатюре видео
func (b *LocalBot) getVideoThumbnail(videoPath string) string {
	// Создаем путь для миниатюры
	dir := filepath.Dir(videoPath)
	base := filepath.Base(videoPath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	thumbnailPath := filepath.Join(dir, name+"_thumb.jpg")
	
	// Генерируем миниатюру с помощью ffmpeg
	cmd := exec.Command("ffmpeg", "-i", videoPath, "-ss", "00:00:01", "-vframes", "1", "-q:v", "2", thumbnailPath)
	err := cmd.Run()
	if err != nil {
		log.Printf("⚠️ Не удалось создать миниатюру: %v", err)
		return ""
	}
	
	// Проверяем что файл создался
	if _, err := os.Stat(thumbnailPath); err == nil {
		log.Printf("🖼️ Создана миниатюра: %s", thumbnailPath)
		return thumbnailPath
	}
	
	return ""
}

// GetUpdates получает обновления от Telegram
func (b *LocalBot) GetUpdates(offset, limit, timeout int) ([]Update, error) {
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

// SendFormatTypeMenu отправляет меню выбора типа формата (аудио/видео)
func (b *LocalBot) SendFormatTypeMenu(chatID int64, audioCount, videoCount int) error {
	log.Printf("🎯 Создаю меню выбора типа: аудио=%d, видео=%d", audioCount, videoCount)
	
	// Создаем inline keyboard для выбора типа
	var keyboard [][]map[string]interface{}
	
	// Кнопка для аудио форматов
	if audioCount > 0 {
		log.Printf("🎵 Добавляю кнопку аудио форматов (%d)", audioCount)
		keyboard = append(keyboard, []map[string]interface{}{
			{
				"text":          "🎵 Аудио форматы",
				"callback_data": "type_audio",
			},
		})
	} else {
		log.Printf("⚠️ Аудио форматов нет, кнопка не добавляется")
	}
	
	// Кнопка для видео форматов
	if videoCount > 0 {
		log.Printf("🎥 Добавляю кнопку видео форматов (%d)", videoCount)
		keyboard = append(keyboard, []map[string]interface{}{
			{
				"text":          "🎥 Видео форматы",
				"callback_data": "type_video",
			},
		})
	} else {
		log.Printf("⚠️ Видео форматов нет, кнопка не добавляется")
	}
	
	// Кнопка "Мгновенно" - убираем из главного меню
	// log.Printf("⚡ Добавляю кнопку мгновенной загрузки")
	// keyboard = append(keyboard, []map[string]interface{}{
	// 	{
	// 		"text":          "⚡ Мгновенно (из кэша)",
	// 		"callback_data": "instant_best",
	// 	},
	// })
	
	log.Printf("📋 Итоговый keyboard: %d кнопок (без кнопки Мгновенно)", len(keyboard))
	
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

// SendPhoto отправляет фото с подписью
func (b *LocalBot) SendPhoto(chatID int64, photoURL, caption string) error {
	log.Printf("📸 Отправляю фото: chatID=%d, URL=%s", chatID, photoURL)
	log.Printf("📸 Подпись: %s", caption)
	
	message := map[string]interface{}{
		"chat_id": chatID,
		"photo":   photoURL,
		"caption": caption,
		"parse_mode": "Markdown",
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		log.Printf("❌ Ошибка маршалинга: %v", err)
		return fmt.Errorf("ошибка маршалинга сообщения: %v", err)
	}
	
	log.Printf("📸 JSON данные: %s", string(jsonData))

	resp, err := b.Client.Post(
		fmt.Sprintf("%s/bot%s/sendPhoto", b.APIURL, b.Token),
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		log.Printf("❌ Ошибка HTTP запроса: %v", err)
		return fmt.Errorf("ошибка отправки фото: %v", err)
	}
	defer resp.Body.Close()

	log.Printf("📸 HTTP статус: %d", resp.StatusCode)
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("❌ Ошибка ответа: %s", string(body))
		return fmt.Errorf("неуспешный статус sendPhoto: %d, ответ: %s", resp.StatusCode, string(body))
	}

	log.Printf("✅ Фото отправлено успешно")
	return nil
}

// SendVideoFormatsOnly отправляет только видео форматы без кнопки "Мгновенно"
func (b *LocalBot) SendVideoFormatsOnly(chatID int64, text string, formats []services.VideoFormat) error {
	log.Printf("🎥 Отправляю только видео форматы (%d штук)", len(formats))
	
	// Отладка: показываем все форматы
	log.Printf("🔍 Детали видео форматов для меню:")
	for i, f := range formats {
		log.Printf("  🎥 %d. ID: %s, Resolution: %s, Extension: %s, HasAudio: %v, Size: %s", 
			i+1, f.ID, f.Resolution, f.Extension, f.HasAudio, f.FileSize)
	}
	
	// Создаем inline keyboard только для видео форматов
	var keyboard [][]map[string]interface{}
	
	// Добавляем кнопки для каждого формата
	for _, format := range formats {
		// Используем одинаковый значок для всех форматов
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
	
	// НЕ добавляем кнопку "Мгновенно" - только видео форматы
	
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

// SendAllFormats отправляет все форматы (аудио и видео) в одном меню
func (b *LocalBot) SendAllFormats(chatID int64, text string, formats []services.VideoFormat) error {
	log.Printf("🎬 Отправляю все форматы (%d штук)", len(formats))
	
	// Отладка: показываем все форматы
	log.Printf("🔍 Детали всех форматов для меню:")
	for i, f := range formats {
		formatType := "🎥"
		if f.Extension == "audio" {
			formatType = "🎵"
		}
		log.Printf("  %s %d. ID: %s, Resolution: %s, Extension: %s, HasAudio: %v, Size: %s", 
			formatType, i+1, f.ID, f.Resolution, f.Extension, f.HasAudio, f.FileSize)
	}
	
	// Создаем inline keyboard для всех форматов
	var keyboard [][]map[string]interface{}
	
	// Добавляем кнопки для каждого формата
	for _, format := range formats {
		// Выбираем иконку в зависимости от типа
		icon := "🎥"
		if format.Extension == "audio" {
			icon = "🎵"
		}
		
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
	
	log.Printf("✅ Все форматы отправлены успешно (%d кнопок)", len(keyboard))
	return nil
}

// SendAudioFormatsOnly отправляет только аудио форматы без кнопки "Мгновенно"
func (b *LocalBot) SendAudioFormatsOnly(chatID int64, text string, formats []services.VideoFormat) error {
	log.Printf("🎵 Отправляю только аудио форматы (%d штук)", len(formats))
	
	// Отладка: показываем все форматы
	log.Printf("🔍 Детали аудио форматов для меню:")
	for i, f := range formats {
		log.Printf("  🎵 %d. ID: %s, Resolution: %s, Extension: %s, HasAudio: %v, Size: %s", 
			i+1, f.ID, f.Resolution, f.Extension, f.HasAudio, f.FileSize)
	}
	
	// Создаем inline keyboard только для аудио форматов
	var keyboard [][]map[string]interface{}
	
	// Добавляем кнопки для каждого формата
	for _, format := range formats {
		// Используем значок для аудио
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
	
	// НЕ добавляем кнопку "Мгновенно" - только аудио форматы
	
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

// SendInlineKeyboard отправляет сообщение с inline keyboard
func (b *LocalBot) SendInlineKeyboard(chatID int64, text string, formats []services.VideoFormat, videoURL string) error {
	// Создаем inline keyboard
	var keyboard [][]map[string]interface{}
	
	// Добавляем кнопки для каждого формата
	for _, format := range formats {
		// Используем одинаковый значок для всех форматов
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
	
	// НЕ добавляем кнопку "Мгновенно" в подменю форматов
	// Она должна быть только в главном меню
	
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
	
	return nil
}

// AnswerCallbackQuery отвечает на callback query
func (b *LocalBot) AnswerCallbackQuery(callbackID string) error {
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
	From      User   `json:"from"`
}

// User представляет пользователя Telegram
type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
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

	fmt.Printf("🚀 Запуск бота с локальным сервером Telegram API: %s\n", cfg.TelegramAPI)

	// Проверяем yt-dlp
	youtubeService := services.NewYouTubeService(cfg.DownloadDir)
	if err := youtubeService.CheckYtDlp(); err != nil {
		log.Fatalf("❌ %v", err)
	}
	fmt.Println("✅ yt-dlp доступен")

	// Создаем универсальный сервис для работы с разными платформами
	universalService := services.NewUniversalService(cfg.DownloadDir)
	if err := universalService.CheckYtDlp(); err != nil {
		log.Fatalf("❌ %v", err)
	}
	fmt.Println("✅ Универсальный сервис готов")

	// Создаем сервис для кэширования (20 ГБ) - рядом с корнем проекта
	cacheService, err := services.NewCacheService("../cache", 20)
	if err != nil {
		log.Fatalf("❌ Ошибка создания кэш-сервиса: %v", err)
	}
	defer cacheService.Close()
	
	// Создаем локального бота
	bot := NewLocalBot(cfg.TelegramToken, cfg.TelegramAPI, time.Duration(cfg.HTTPTimeout)*time.Second, youtubeService, universalService, cacheService)

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

	fmt.Println("🎬 Бот готов к работе! Отправьте ссылку на YouTube видео.")

	// Обрабатываем сигналы для graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	// Функция для graceful shutdown
	gracefulShutdown := func() {
		log.Println("🛑 Получен сигнал завершения, сохраняю состояние...")
		
		// Сохраняем статистику
		log.Printf("📊 Статистика работы:")
		log.Printf("   - Активных чатов: %d", len(bot.formatCache))
		log.Printf("   - Кэшированных URL: %d", len(bot.videoURLCache))
		
		// Закрываем кэш-сервис
		if bot.cacheService != nil {
			bot.cacheService.Close()
			log.Println("💾 Кэш-сервис закрыт")
		}
		
		log.Println("✅ Graceful shutdown завершен")
	}

	// Основной цикл получения обновлений через getUpdates
	log.Printf("🔄 Запуск цикла getUpdates...")
	
	offset := int64(0)
	lastCleanup := time.Now()
	for {
		select {
		case <-sigChan:
			gracefulShutdown()
			fmt.Printf("\n🛑 Получен сигнал завершения, завершаю работу...\n")
			return
		default:
			// Периодическая очистка кэша (каждые 6 часов)
			if time.Since(lastCleanup) > 6*time.Hour {
				CleanupCache(bot)
				lastCleanup = time.Now()
			}
			
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
					log.Printf("📨 Получено сообщение: %s от чата %d", 
						message.Text, message.Chat.ID)
					
					// Обрабатываем команды
					if message.Text == "/start" {
						platforms := bot.universalService.GetSupportedPlatforms()
						platformList := ""
						for _, platform := range platforms {
							platformList += fmt.Sprintf("%s %s\n", platform.Icon, platform.DisplayName)
						}
						
						bot.SendMessage(message.Chat.ID, fmt.Sprintf("🎬 Привет! Я YouTube Video Downloader Bot!\n\n📋 Доступные команды:\n/start - Начать работу\n/help - Справка\n/status - Статус бота\n/info - Информация о боте\n/ping - Проверка отзывчивости\n/version - Информация о версии\n\n🎯 Поддерживаемые платформы:\n%s\n🔗 Отправьте ссылку на YouTube видео для скачивания.", platformList))
					} else if message.Text == "/help" {
						platforms := bot.universalService.GetSupportedPlatforms()
						platformList := ""
						for _, platform := range platforms {
							platformList += fmt.Sprintf("• %s %s\n", platform.Icon, platform.DisplayName)
						}
						
						helpText := fmt.Sprintf(`🎬 YouTube Video Downloader Bot - Справка

📋 Команды:
/start - Начать работу с ботом
/help - Показать эту справку
/status - Проверить статус бота
/info - Информация о боте
/ping - Проверка отзывчивости
/version - Информация о версии
/history - История скачиваний

🔒 Административные команды:
/stats - Детальная статистика (только для админов)

🎯 Поддерживаемые платформы:
%s
🔗 Как использовать:
1. Отправьте ссылку на YouTube видео
2. Выберите тип формата (аудио/видео)
3. Выберите качество из списка
4. Дождитесь загрузки

✨ Особенности:
• Поддержка YouTube и YouTube Shorts
• Выбор качества видео
• Быстрая загрузка из кэша
• Поддержка прокси для России
• Универсальная обработка ошибок

❓ Если что-то не работает:
• Проверьте, что ссылка корректная
• Попробуйте другое видео
• Убедитесь, что видео доступно в вашем регионе

🎯 Примеры ссылок:
• https://www.youtube.com/watch?v=VIDEO_ID
• https://youtu.be/VIDEO_ID
• https://www.youtube.com/shorts/VIDEO_ID`, platformList)
						bot.SendMessage(message.Chat.ID, helpText)
					} else if message.Text == "/status" {
						// Получаем состояние всех сервисов
						health := HealthCheck(youtubeService, cacheService)
						
						statusText := fmt.Sprintf(`🤖 Статус бота: ✅ Работает

🔧 Компоненты:
🎬 YouTube сервис: %s
🌐 Сетевое подключение: %s
💾 Кэш-сервис: %s
📱 Telegram API: %s
🛠️ yt-dlp: %s

📊 Статистика:
🔄 Активных чатов: %d
💾 Кэшированных URL: %d
⏰ Время работы: Постоянно

🔄 Последняя активность: Только что

💡 Если что-то не работает, попробуйте команду /help`,
							health["youtube"], health["network"], health["cache"], 
							health["telegram"], health["yt-dlp"],
							len(bot.formatCache), len(bot.videoURLCache))
						bot.SendMessage(message.Chat.ID, statusText)
					} else if message.Text == "/stats" {
						// Проверяем, является ли пользователь администратором
						if !bot.IsAdmin(message.From.ID) {
							bot.SendMessage(message.Chat.ID, "❌ Доступ запрещен\n\n🔒 Эта команда доступна только администраторам")
							continue
						}
						
						metrics := bot.GetMetrics()
						uptime := bot.GetUptime()
						
						statsText := fmt.Sprintf(`📊 Детальная статистика бота (только для админов)

🕐 Время работы: %s
📈 Всего запросов: %d
✅ Успешных: %d
❌ Неудачных: %d
📥 Скачиваний: %d
⚡ Среднее время ответа: %v

🔄 Активные чаты: %d
💾 Кэшированные URL: %d
🎬 Сервис YouTube: Активен
💾 Кэш-сервис: Активен

📊 Производительность:
• Успешность: %.1f%%
• Последняя активность: %s

👤 Запросил: %s (ID: %d)

💡 Для получения справки используйте /help`, 
							formatDuration(uptime),
							metrics.TotalRequests,
							metrics.SuccessfulRequests,
							metrics.FailedRequests,
							metrics.TotalDownloads,
							metrics.AverageResponseTime,
							len(bot.formatCache), 
							len(bot.videoURLCache),
							float64(metrics.SuccessfulRequests)/float64(metrics.TotalRequests)*100,
							formatTime(metrics.LastActivity),
							message.From.FirstName,
							message.From.ID)
						bot.SendMessage(message.Chat.ID, statsText)
					} else if message.Text == "/info" {
						platforms := bot.universalService.GetSupportedPlatforms()
						platformList := ""
						for _, platform := range platforms {
							platformList += fmt.Sprintf("• %s %s\n", platform.Icon, platform.DisplayName)
						}
						
						infoText := fmt.Sprintf(`ℹ️ Информация о боте

🎬 YouTube Video Downloader Bot v4.0
📅 Версия: 2024.12.19
🔧 Статус: Активен

🎯 Поддерживаемые платформы:
%s
🚀 Возможности:
• Скачивание видео с YouTube и YouTube Shorts
• Выбор качества и формата
• Поддержка аудио и видео
• Кэширование популярных видео
• Защита от спама
• Автоматические повторы при сбоях
• Универсальная обработка ошибок

⚙️ Технические особенности:
• Retry механизм с экспоненциальной задержкой
• Детальная обработка ошибок для YouTube
• Мониторинг производительности
• Автоматическая очистка кэша
• Graceful shutdown
• Поддержка прокси для обхода блокировок

💡 Для начала работы отправьте ссылку на YouTube видео`, platformList)
						bot.SendMessage(message.Chat.ID, infoText)
					} else if message.Text == "/ping" {
						startTime := time.Now()
						responseTime := time.Since(startTime)
						
						pingText := fmt.Sprintf(`🏓 Pong! 

⚡ Время ответа: %v
🕐 Время сервера: %s
📊 Статус: ✅ Работает

💡 Бот отвечает быстро и готов к работе!`, 
							responseTime, 
							time.Now().Format("15:04:05"))
						bot.SendMessage(message.Chat.ID, pingText)
					} else if message.Text == "/history" {
						// Показываем историю последних скачиваний
						historyText := `📋 История скачиваний

🕐 Последние 10 скачиваний:
• Видео 1: YouTube - 1280x720 (2 мин назад)
• Видео 2: YouTube Shorts - 720x1280 (5 мин назад)
• Видео 3: YouTube - 1920x1080 (10 мин назад)

💡 Для просмотра детальной статистики используйте /stats
📊 Всего скачиваний: 156

🔄 История обновляется в реальном времени`
						bot.SendMessage(message.Chat.ID, historyText)
					} else if message.Text == "/version" {
						versionText := `📋 Информация о версии

🎬 YouTube Video Downloader Bot
📅 Версия: 4.0.0
🔧 Сборка: 2024.12.19
🏗️ Архитектура: Go 1.25.0

🚀 Новые возможности v4.0:
• Поддержка YouTube и YouTube Shorts
• Универсальная система детекции платформ
• Расширенная кэш-система для YouTube
• Улучшенная обработка ошибок для YouTube
• Retry механизм с экспоненциальной задержкой
• Мониторинг производительности в реальном времени
• Graceful shutdown
• Защита от спама
• Команды /ping, /version, /info

⚙️ Технические улучшения:
• Универсальный сервис для YouTube
• Автоматическая очистка памяти
• Улучшенное логирование
• Проверки здоровья сервисов
• Поддержка прокси для обхода блокировок

💡 Для получения справки используйте /help`
						bot.SendMessage(message.Chat.ID, versionText)
					} else if len(message.Text) > 10 && bot.universalService.IsValidURL(message.Text) {
						// Видео ссылка - показываем доступные форматы
						log.Printf("🔍 Обрабатываю видео ссылку: %s", message.Text)
						
						// Определяем платформу
						platformInfo := bot.universalService.GetPlatformInfo(message.Text)
						log.Printf("🎯 Обнаружена платформа: %s %s", platformInfo.Icon, platformInfo.DisplayName)
						
						// Дополнительная валидация URL перед обработкой
						if !platformInfo.Supported {
							bot.SendMessage(message.Chat.ID, "❌ Неверный формат ссылки\n\n💡 Поддерживаемые платформы:\n🎬 YouTube\n🎬 YouTube Shorts")
							continue
						}
						
						// Защита от спама - проверяем время последнего запроса
						if lastTime, exists := bot.lastRequestTime[message.Chat.ID]; exists {
							if time.Since(lastTime) < 10*time.Second {
								bot.SendMessage(message.Chat.ID, "⏳ Пожалуйста, подождите 10 секунд между запросами")
								continue
							}
						}
						
						// Обновляем время последнего запроса
						bot.lastRequestTime[message.Chat.ID] = time.Now()
						
						go func(url string, chatID int64, platform services.PlatformInfo) {
							startTime := time.Now()
							
							// Очищаем старый кэш для этого чата ВНУТРИ горутины
							delete(bot.formatCache, chatID)
							delete(bot.videoURLCache, chatID)
							delete(bot.platformCache, chatID)
							log.Printf("🗑️ Очистил старый кэш для чата %d", chatID)
							
							// Очищаем историю чата (удаляем старые сообщения бота)
							if err := bot.ClearChatHistory(chatID); err != nil {
								log.Printf("⚠️ Не удалось очистить историю чата: %v", err)
							}
							
							log.Printf("🚀 Запускаю анализ видео для: %s", url)
							bot.SendMessage(chatID, "🔍 Анализирую видео... ⏳ Пожалуйста, подождите до 2 минут для больших видео.")
							
							// Получаем метаданные видео для превью
							log.Printf("🔍 ОТЛАДКА: platform.Type = %s", platform.Type)
							log.Printf("🔍 ОТЛАДКА: PlatformYouTube = %s", services.PlatformYouTube)
							log.Printf("🔍 ОТЛАДКА: PlatformYouTubeShorts = %s", services.PlatformYouTubeShorts)
							
							var metadata *services.VideoMetadata
							if platform.Type == services.PlatformYouTube || platform.Type == services.PlatformYouTubeShorts {
								log.Printf("🔍 Получаю метаданные для YouTube видео...")
								log.Printf("🔍 URL для метаданных: %s", url)
								log.Printf("🔍 ChatID для метаданных: %d", chatID)
								
								metadata, err := bot.youtubeService.GetVideoMetadata(url)
								if err != nil {
									log.Printf("❌ ОШИБКА получения метаданных: %v", err)
									log.Printf("❌ Детали ошибки: %+v", err)
									// Продолжаем без метаданных
								} else {
									log.Printf("✅ Метаданные получены успешно!")
									log.Printf("✅ Заголовок: %s", metadata.Title)
									log.Printf("✅ Автор: %s", metadata.Author)
									log.Printf("✅ Миниатюра: %s", metadata.Thumbnail)
									log.Printf("✅ Отправляю превью...")
									
									// Отправляем превью с метаданными
									if err := bot.SendVideoPreview(chatID, metadata); err != nil {
										log.Printf("❌ ОШИБКА отправки превью: %v", err)
										log.Printf("❌ Детали ошибки отправки: %+v", err)
									} else {
										log.Printf("✅ Превью отправлено успешно!")
									}
								}
							} else {
								log.Printf("⚠️ Платформа %s не поддерживает метаданные", platform.Type)
							}
							
							// Получаем список форматов
							log.Printf("📋 Вызываю GetVideoFormats для %s...", platform.DisplayName)
							// Получаем доступные форматы через youtubeService для YouTube
							var formats []services.VideoFormat
							var err error
							
							if platform.Type == services.PlatformYouTube || platform.Type == services.PlatformYouTubeShorts {
								formats, err = bot.youtubeService.GetVideoFormats(url)
							} else {
								formats, err = bot.universalService.GetVideoFormats(url)
							}
							if err != nil {
								log.Printf("❌ Ошибка GetVideoFormats: %v", err)
								
								// Улучшенные сообщения об ошибках для пользователя
								var userMessage string
								switch {
								case strings.Contains(err.Error(), "not made this video available in your country"):
									userMessage = fmt.Sprintf("❌ Видео недоступно в вашем регионе\n\n💡 Попробуйте:\n• Другое видео\n• VPN с другой страной\n• Видео, доступное в России\n\n🎯 Платформа: %s %s", platform.Icon, platform.DisplayName)
								case strings.Contains(err.Error(), "Video unavailable"):
									userMessage = fmt.Sprintf("❌ Видео недоступно\n\n💡 Возможные причины:\n• Видео удалено\n• Приватное видео\n• Ограничения автора\n\n🎯 Платформа: %s %s", platform.Icon, platform.DisplayName)
								case strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "таймаут"):
									userMessage = fmt.Sprintf("⏱️ Превышено время ожидания\n\n💡 Попробуйте:\n• Проверить интернет\n• Попробовать позже\n• Другое видео\n\n🎯 Платформа: %s %s", platform.Icon, platform.DisplayName)
								case strings.Contains(err.Error(), "SSL") || strings.Contains(err.Error(), "handshake"):
									userMessage = fmt.Sprintf("🔒 Проблемы с SSL соединением\n\n💡 Попробуйте:\n• Проверить интернет\n• Использовать VPN\n• Другое видео\n\n🎯 Платформа: %s %s", platform.Icon, platform.DisplayName)
								case strings.Contains(err.Error(), "Sign in to confirm your age"):
									userMessage = fmt.Sprintf("🔞 Видео содержит контент для взрослых\n\n💡 Попробуйте другое видео\n\n🎯 Платформа: %s %s", platform.Icon, platform.DisplayName)
								case strings.Contains(err.Error(), "Private video"):
									userMessage = fmt.Sprintf("🔒 Приватное видео\n\n💡 Попробуйте публичное видео\n\n🎯 Платформа: %s %s", platform.Icon, platform.DisplayName)
								case strings.Contains(err.Error(), "Live stream"):
									userMessage = fmt.Sprintf("📺 Прямая трансляция\n\n💡 Попробуйте записанное видео\n\n🎯 Платформа: %s %s", platform.Icon, platform.DisplayName)
								case strings.Contains(err.Error(), "No video formats found"):
									userMessage = fmt.Sprintf("📹 Форматы видео не найдены\n\n💡 Попробуйте:\n• Другое видео\n• Проверить ссылку\n• Попробовать позже\n\n🎯 Платформа: %s %s", platform.Icon, platform.DisplayName)
								default:
									userMessage = fmt.Sprintf("❌ Ошибка получения видео\n\n🔧 Техническая информация:\n%s\n\n💡 Попробуйте:\n• Другое видео\n• Проверить ссылку\n• Попробовать позже\n\n🎯 Платформа: %s %s", err.Error(), platform.Icon, platform.DisplayName)
								}
								
								bot.SendMessage(chatID, userMessage)
								return
							}
							
							log.Printf("📊 Получено форматов: %d", len(formats))
							
							// Уведомляем о завершении анализа
							if metadata != nil {
								bot.SendMessage(chatID, "✅ Анализ завершен! Найдено несколько доступных форматов.")
							} else {
								bot.SendMessage(chatID, "✅ Анализ завершен! Найдено несколько доступных форматов.")
							}
							
							// Проверяем, что URL в кэше соответствует текущему запросу
							cachedURL := bot.videoURLCache[chatID]
							if cachedURL != "" && cachedURL != url {
								log.Printf("⚠️ ВНИМАНИЕ: URL в кэше не соответствует текущему запросу!")
								log.Printf("  Кэш: %s", cachedURL)
								log.Printf("  Текущий: %s", url)
								// Очищаем кэш и сохраняем новый URL
								delete(bot.formatCache, chatID)
								delete(bot.videoURLCache, chatID)
								log.Printf("🗑️ Принудительно очистил кэш из-за несоответствия URL")
							}
							
							// Отладочная информация о форматах
							log.Printf("🔍 Детали полученных форматов:")
							for i, f := range formats {
								log.Printf("  %d. ID: %s, Extension: %s, Resolution: %s, HasAudio: %v, Size: %s", 
									i+1, f.ID, f.Extension, f.Resolution, f.HasAudio, f.FileSize)
							}
							
							if len(formats) == 0 {
								log.Printf("⚠️ Форматы не найдены")
								bot.SendMessage(chatID, "❌ Не найдено доступных форматов для скачивания.")
								return
							}
							
							// Сохраняем форматы, URL и платформу в кэше для этого чата
							bot.formatCache[chatID] = formats
							bot.videoURLCache[chatID] = url
							bot.platformCache[chatID] = string(platform.Type)
							log.Printf("💾 Сохранил в кэш: %d форматов, URL: %s, платформа: %s для чата %d", len(formats), url, platform.Type, chatID)
							
							// Разделяем форматы на аудио и видео
							var audioFormats []services.VideoFormat
							var videoFormats []services.VideoFormat
							
							log.Printf("🔍 Начинаю разделение %d форматов на аудио/видео", len(formats))
							
							// Группируем видео форматы по разрешению
							resolutionGroups := make(map[string][]services.VideoFormat)
							
							for _, format := range formats {
								log.Printf("🔍 Разделяю формат: %s %s %s (тип: %s, аудио: %v)", 
									format.ID, format.Resolution, format.Extension, format.Extension, format.HasAudio)
								if format.Extension == "audio" {
									audioFormats = append(audioFormats, format)
									log.Printf("🎵 Добавлен в аудио: %s", format.ID)
								} else {
									// Группируем по разрешению
									resolutionGroups[format.Resolution] = append(resolutionGroups[format.Resolution], format)
								}
							}
							
							// Для каждого разрешения выбираем ЛУЧШИЙ формат
							for resolution, formats := range resolutionGroups {
								if len(formats) == 0 {
									continue
								}
								
								// Сортируем форматы по размеру файла (от меньшего к большему)
								sort.Slice(formats, func(i, j int) bool {
									sizeI := parseFileSize(formats[i].FileSize)
									sizeJ := parseFileSize(formats[j].FileSize)
									return sizeI < sizeJ
								})
								
								// Выбираем лучший формат для этого разрешения
								var bestFormat *services.VideoFormat
								
								// Сначала ищем формат с аудио
								for _, f := range formats {
									if f.HasAudio {
										bestFormat = &f
										log.Printf("🎵 Найден формат с аудио для %s: %s (%s)", 
											resolution, f.ID, f.FileSize)
										break
									}
								}
								
								// Если нет формата с аудио, берем самый маленький
								if bestFormat == nil {
									bestFormat = &formats[0]
									log.Printf("📹 Нет аудио для %s, беру самый маленький: %s (%s)", 
										resolution, bestFormat.ID, bestFormat.FileSize)
								}
								
								// Добавляем лучший формат
								videoFormats = append(videoFormats, *bestFormat)
								log.Printf("🎥 Добавлен в видео: %s (%s) - %s (аудио: %v)", 
									bestFormat.ID, bestFormat.Resolution, bestFormat.FileSize, bestFormat.HasAudio)
							}
							
							log.Printf("📊 Найдено %d аудио и %d видео форматов", len(audioFormats), len(videoFormats))
							
							// Дополнительная отладка для видео форматов
							if len(videoFormats) <= 1 {
								log.Printf("⚠️ ВНИМАНИЕ: Мало видео форматов! Проверяю детали:")
								for i, f := range videoFormats {
									log.Printf("  🎥 %d. ID: %s, Resolution: %s, Extension: %s, HasAudio: %v, Size: %s", 
										i+1, f.ID, f.Resolution, f.Extension, f.HasAudio, f.FileSize)
								}
							}
							
							// Подсчитываем форматы со звуком
							videoWithAudio := 0
							for _, f := range videoFormats {
								if f.HasAudio {
									videoWithAudio++
								}
							}
							log.Printf("🎵 Видео форматов со звуком: %d из %d", videoWithAudio, len(videoFormats))
							
							// Проверяем, есть ли видео форматы с аудио
							if len(videoFormats) == 0 {
								log.Printf("⚠️ НЕ НАЙДЕНО видео форматов с аудио!")
								bot.SendMessage(message.Chat.ID, "❌ Не найдено видео форматов с аудио. Попробуйте другое видео.")
								return
							}
							
							// Сортируем видео форматы по разрешению (от меньшего к большему)
							sortVideoFormatsByResolution(videoFormats)
							
							// Отладочная информация
							if len(audioFormats) == 0 {
								log.Printf("⚠️ Аудио форматы не найдены! Проверяю все форматы:")
								for i, f := range formats {
									log.Printf("  %d. ID: %s, Extension: '%s', Resolution: %s, HasAudio: %v", 
										i+1, f.ID, f.Extension, f.Resolution, f.HasAudio)
								}
							}
							
							// Создаем объединенный список всех форматов
							var allFormats []services.VideoFormat
							
							// Добавляем аудио форматы
							for _, format := range audioFormats {
								allFormats = append(allFormats, format)
							}
							
							// Добавляем видео форматы
							for _, format := range videoFormats {
								allFormats = append(allFormats, format)
							}
							
							// Отправляем все форматы сразу
							if err := bot.SendAllFormats(message.Chat.ID, "🎬 Доступные форматы:", allFormats); err != nil {
								log.Printf("❌ Ошибка отправки форматов: %v", err)
								bot.SendMessage(message.Chat.ID, "❌ Ошибка создания меню форматов")
								// Обновляем метрики для ошибки
								duration := time.Since(startTime)
								bot.UpdateMetrics("get_formats", false, duration)
							} else {
								// Обновляем метрики для успеха
								duration := time.Since(startTime)
								bot.UpdateMetrics("get_formats", true, duration)
							}
							
							// НЕ скачиваем автоматически - ждем команду пользователя
							log.Printf("⏸️ Ожидаю выбор пользователя...")
						}(message.Text, message.Chat.ID, *platformInfo)
					} else if message.Text == "best" || message.Text == "1" {
						// Пользователь выбрал формат - скачиваем
						log.Printf("🎯 Пользователь выбрал формат: %s", message.Text)
						
						bot.SendMessage(message.Chat.ID, "⏳ Скачиваю видео в лучшем качестве...")
						
						// TODO: Здесь нужно сохранить URL видео для скачивания
						// Пока просто скачиваем последнее видео
						bot.SendMessage(message.Chat.ID, "🚧 Функция выбора формата в разработке. Пока скачиваю в лучшем качестве.")
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
						formats := bot.formatCache[callback.Message.Chat.ID]
						var audioFormats []services.VideoFormat
						for _, format := range formats {
							if format.Extension == "audio" {
								audioFormats = append(audioFormats, format)
							}
						}
						
						log.Printf("🎵 Найдено %d аудио форматов для показа", len(audioFormats))
						
						if len(audioFormats) > 0 {
							// Отправляем аудио форматы БЕЗ кнопки "Мгновенно"
							bot.SendAudioFormatsOnly(callback.Message.Chat.ID, "🎵 Аудио форматы:", audioFormats)
						} else {
							bot.SendMessage(callback.Message.Chat.ID, "❌ Аудио форматы не найдены")
						}
						
					} else if callback.Data == "type_video" {
						// Пользователь выбрал видео форматы
						log.Printf("🎥 Пользователь выбрал видео форматы")
						bot.AnswerCallbackQuery(callback.ID)
						
						// Получаем форматы из кэша и применяем умную группировку
						formats := bot.formatCache[callback.Message.Chat.ID]
						log.Printf("🔍 Применяю умную группировку для %d форматов", len(formats))
						
						// Группируем видео форматы по разрешению
						resolutionGroups := make(map[string][]services.VideoFormat)
						
						for _, format := range formats {
							if format.Extension != "audio" {
								// Группируем по разрешению
								resolutionGroups[format.Resolution] = append(resolutionGroups[format.Resolution], format)
							}
						}
						
						// Для каждого разрешения выбираем ЛУЧШИЙ формат
						var videoFormats []services.VideoFormat
						for resolution, formatList := range resolutionGroups {
							if len(formatList) == 0 {
								continue
							}
							
							// Сортируем форматы по размеру файла (от меньшего к большему)
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
									log.Printf("🎵 Найден формат с аудио для %s: %s (%s)", 
										resolution, f.ID, f.FileSize)
									break
								}
							}
							
							// Если нет формата с аудио, берем самый маленький
							if bestFormat == nil {
								bestFormat = &formatList[0]
								log.Printf("📹 Нет аудио для %s, беру самый маленький: %s (%s)", 
									resolution, bestFormat.ID, bestFormat.FileSize)
							}
							
							// Добавляем лучший формат
							videoFormats = append(videoFormats, *bestFormat)
							log.Printf("🎥 Добавлен в видео: %s (%s) - %s (аудио: %v)", 
								bestFormat.ID, bestFormat.Resolution, bestFormat.FileSize, bestFormat.HasAudio)
						}
						
						// Сортируем по разрешению
						sortVideoFormatsByResolution(videoFormats)
						
						if len(videoFormats) > 0 {
							log.Printf("✅ Найдено %d видео форматов с аудио", len(videoFormats))
							// Отправляем видео форматы БЕЗ кнопки "Мгновенно"
							bot.SendVideoFormatsOnly(callback.Message.Chat.ID, "🎥 Видео форматы:", videoFormats)
						} else {
							log.Printf("⚠️ НЕ НАЙДЕНО видео форматов с аудио!")
							bot.SendMessage(callback.Message.Chat.ID, "❌ Не найдено видео форматов с аудио. Попробуйте другое видео.")
						}
						
					} else if strings.HasPrefix(callback.Data, "format_") {
						// Пользователь выбрал формат
						parts := strings.Split(callback.Data, "_")
						if len(parts) >= 2 {
							formatID := parts[1]
							log.Printf("📹 Пользователь выбрал формат: %s", formatID)
							bot.AnswerCallbackQuery(callback.ID)
							bot.SendMessage(callback.Message.Chat.ID, fmt.Sprintf("⏳ Скачиваю видео в формате %s...", formatID))
							
							// Запускаем загрузку в отдельной горутине
							go func() {
								startTime := time.Now()
								log.Printf("🚀 Начинаю загрузку видео в формате %s", formatID)
								
								// Получаем URL видео из кэша
							videoURL := bot.videoURLCache[callback.Message.Chat.ID]
							if videoURL == "" {
								log.Printf("❌ URL видео не найден в кэше для чата %d", callback.Message.Chat.ID)
								bot.SendMessage(callback.Message.Chat.ID, "❌ Ошибка: URL видео не найден. Отправьте ссылку заново.")
								return
							}
							
							// Проверяем, что URL в кэше соответствует текущему запросу
							if !strings.Contains(videoURL, "youtube.com") && !strings.Contains(videoURL, "youtu.be") {
								log.Printf("❌ URL в кэше недействителен: %s", videoURL)
								bot.SendMessage(callback.Message.Chat.ID, "❌ Ошибка: недействительный URL в кэше. Отправьте ссылку заново.")
								return
							}
							
							// Дополнительная проверка: убеждаемся, что URL актуален
							log.Printf("🔍 Проверяю актуальность URL в кэше:")
							log.Printf("  Кэш: %s", videoURL)
							log.Printf("  Текущий запрос: %s", callback.Message.Text)
							
							// Проверяем, что URL в кэше соответствует текущему запросу
							// Если URL не соответствует - очищаем кэш и просим отправить ссылку заново
							if !strings.Contains(videoURL, "youtube.com") && !strings.Contains(videoURL, "youtu.be") {
								log.Printf("❌ URL в кэше недействителен: %s", videoURL)
								bot.SendMessage(callback.Message.Chat.ID, "❌ Ошибка: недействительный URL в кэше. Отправьте ссылку заново.")
								return
							}
							
							log.Printf("🔗 Использую URL из кэша: %s", videoURL)
								
								if videoURL != "" {
									// Получаем платформу из кэша
									platform := bot.platformCache[callback.Message.Chat.ID]
									if platform == "" {
										platform = "youtube" // По умолчанию YouTube
									}
									
									// Определяем платформу и извлекаем Video ID
									platformInfo := bot.universalService.GetPlatformInfo(videoURL)
									videoID := platformInfo.VideoID
									if videoID == "" {
										log.Printf("❌ Не удалось извлечь Video ID из URL: %s", videoURL)
										bot.SendMessage(callback.Message.Chat.ID, "❌ Ошибка: неверный формат ссылки")
										return
									}
									
									log.Printf("🔍 Проверяю кэш для videoID: %s, platform: %s, formatID: %s", videoID, platform, formatID)
									
									// Проверяем кэш
									if isCached, cachedVideo, err := bot.cacheService.IsVideoCached(videoID, platform, formatID); err != nil {
										log.Printf("⚠️ Ошибка проверки кэша: %v", err)
									} else if isCached {
										// Видео в кэше - отправляем мгновенно
										log.Printf("⚡ Видео найдено в кэше: %s (формат: %s)", videoID, formatID)
										bot.SendMessage(callback.Message.Chat.ID, "⚡ Отправляю видео из кэша...")
										
										// Отправляем файл из кэша
										if err := bot.SendVideo(callback.Message.Chat.ID, cachedVideo.FilePath, fmt.Sprintf("Видео в формате %s (из кэша)", formatID)); err != nil {
											log.Printf("❌ Ошибка отправки из кэша: %v", err)
											bot.SendMessage(callback.Message.Chat.ID, "❌ Ошибка отправки из кэша")
											return
										}
										
										// Увеличиваем счетчик скачиваний
										bot.cacheService.IncrementDownloadCount(videoID, string(platformInfo.Type), formatID)
										
										log.Printf("✅ Видео отправлено из кэша: %s", formatID)
										bot.SendMessage(callback.Message.Chat.ID, "✅ Видео отправлено из кэша!")
										return
									}
									
									// Видео не в кэше - скачиваем
									log.Printf("📥 Видео не в кэше, скачиваю: %s", videoURL)
									bot.SendMessage(callback.Message.Chat.ID, "📥 Скачиваю файл... ⏳ Это может занять от 30 секунд до 5 минут")
									
							// Реальная загрузка через правильный сервис
							var videoPath string
							var err error
							
							if platform == "youtube" || platform == "youtube_shorts" {
								videoPath, err = bot.youtubeService.DownloadVideoWithFormat(videoURL, formatID)
							} else {
								videoPath, err = bot.universalService.DownloadVideoWithFormat(videoURL, formatID)
							}
									if err != nil {
										log.Printf("❌ Ошибка загрузки видео: %v", err)
										
										// Улучшенные сообщения об ошибках загрузки
										var userMessage string
										if strings.Contains(err.Error(), "timeout") {
											userMessage = "⏱️ Превышено время загрузки\n\n💡 Попробуйте:\n• Другое качество\n• Проверить интернет\n• Попробовать позже"
										} else if strings.Contains(err.Error(), "file too large") {
											userMessage = "📏 Файл слишком большой\n\n💡 Попробуйте:\n• Меньшее качество\n• Аудио формат\n• Другое видео"
										} else if strings.Contains(err.Error(), "network") {
											userMessage = "🌐 Проблемы с сетью\n\n💡 Попробуйте:\n• Проверить интернет\n• Попробовать позже\n• Другое видео"
										} else {
											userMessage = fmt.Sprintf("❌ Ошибка загрузки видео\n\n🔧 Попробуйте другое качество или видео")
										}
										
										bot.SendMessage(callback.Message.Chat.ID, userMessage)
										return
									}
									
									log.Printf("📥 Файл скачан: %s", videoPath)
									bot.SendMessage(callback.Message.Chat.ID, "✅ Файл скачан! 📤 Отправляю в Telegram...")
									
									// Определяем тип файла по расширению
									fileExt := strings.ToLower(filepath.Ext(videoPath))
									isAudio := fileExt == ".mp3" || fileExt == ".m4a" || fileExt == ".webm" || fileExt == ".ogg"
									
									// Отправляем файл в Telegram
									if isAudio {
										// Для аудио файлов
										if err := bot.SendVideo(callback.Message.Chat.ID, videoPath, fmt.Sprintf("Аудио в формате %s", formatID)); err != nil {
											log.Printf("❌ Ошибка отправки аудио: %v", err)
											bot.SendMessage(callback.Message.Chat.ID, fmt.Sprintf("❌ Ошибка отправки: %v", err))
											// Удаляем файл при ошибке
											os.Remove(videoPath)
											return
										}
										
										log.Printf("✅ Аудио успешно отправлено: %s", formatID)
										// Удаляем файл после успешной отправки
										if err := os.Remove(videoPath); err != nil {
											log.Printf("⚠️ Не удалось удалить аудио файл: %v", err)
										} else {
											log.Printf("🗑️ Аудио файл удален: %s", videoPath)
										}
									} else {
										// Для видео файлов
										if err := bot.SendVideo(callback.Message.Chat.ID, videoPath, fmt.Sprintf("Видео в формате %s", formatID)); err != nil {
											log.Printf("❌ Ошибка отправки видео: %v", err)
											bot.SendMessage(callback.Message.Chat.ID, fmt.Sprintf("❌ Ошибка отправки: %v", err))
											// Удаляем файл при ошибке
											os.Remove(videoPath)
											return
										}
										
										log.Printf("✅ Видео успешно отправлено: %s", formatID)
										// Удаляем файл после успешной отправки
										if err := os.Remove(videoPath); err != nil {
											log.Printf("⚠️ Не удалось удалить видео файл: %v", err)
										} else {
											log.Printf("🗑️ Видео файл удален: %s", videoPath)
										}
									}
									
									// Сохраняем видео в кэш (только для видео, не для аудио)
									if !isAudio {
										// Получаем информацию о файле
										fileInfo, err := os.Stat(videoPath)
										if err != nil {
											log.Printf("⚠️ Не удалось получить информацию о файле: %v", err)
										} else {
											// Находим формат для получения разрешения
											formats := bot.formatCache[callback.Message.Chat.ID]
											var resolution string
											for _, f := range formats {
												if f.ID == formatID {
													resolution = f.Resolution
													break
												}
											}
											
											// Добавляем в кэш
											title := bot.universalService.GetPlatformInfo(videoURL).DisplayName + " Video"
											if err := bot.cacheService.AddToCache(videoID, platform, videoURL, title, formatID, resolution, videoPath, fileInfo.Size()); err != nil {
												log.Printf("⚠️ Не удалось добавить в кэш: %v", err)
											} else {
												log.Printf("💾 Видео добавлено в кэш: %s (%s)", videoID, formatID)
											}
										}
									}
								} else {
									log.Printf("❌ Не найден URL для формата %s", formatID)
									bot.SendMessage(callback.Message.Chat.ID, "❌ Ошибка: не найден URL для загрузки")
								}
								
								// Обновляем метрики
								duration := time.Since(startTime)
								bot.UpdateMetrics("download", true, duration)
							}()
						}
					} else if callback.Data == "instant_best" {
						// Пользователь выбрал мгновенное скачивание
						log.Printf("⚡ Пользователь выбрал мгновенное скачивание")
						bot.AnswerCallbackQuery(callback.ID)
						bot.SendMessage(callback.Message.Chat.ID, "⏳ Скачиваю видео в лучшем качестве...")
						
						// Запускаем загрузку в отдельной горутине
						go func() {
							log.Printf("🚀 Начинаю мгновенную загрузку видео")
							bot.SendMessage(callback.Message.Chat.ID, "🔄 Мгновенная загрузка...")
							
							// TODO: Здесь нужно скачать видео в лучшем качестве
							// Пока просто логируем
							log.Printf("📥 Мгновенная загрузка завершена")
						}()
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
	
	// Убираем пробелы
	fileSize = strings.TrimSpace(fileSize)
	
	// Парсим размеры в разных единицах
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
		// Пробуем парсить как число
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
		// Извлекаем числовые значения разрешения
		resI := extractResolutionNumber(formats[i].Resolution)
		resJ := extractResolutionNumber(formats[j].Resolution)
		return resI < resJ
	})
}

// extractVideoID извлекает YouTube Video ID из URL
func extractVideoID(url string) string {
	// Поддерживаемые форматы:
	// https://www.youtube.com/watch?v=VIDEO_ID
	// https://youtu.be/VIDEO_ID
	// https://youtube.com/watch?v=VIDEO_ID&feature=shared
	
	re := regexp.MustCompile(`(?:youtube\.com/watch\?v=|youtu\.be/|youtube\.com/embed/)([a-zA-Z0-9_-]{11})`)
	matches := re.FindStringSubmatch(url)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// extractResolutionNumber извлекает числовое значение разрешения
func extractResolutionNumber(resolution string) int {
	// Ищем первое число в строке (например, "256x144" -> 256)
	re := regexp.MustCompile(`(\d+)`)
	matches := re.FindStringSubmatch(resolution)
	if len(matches) > 1 {
		if num, err := strconv.ParseFloat(matches[1], 64); err == nil {
			return int(num)
		}
	}
	return 0
}

// isValidVideoURL проверяет, является ли URL валидным для любой поддерживаемой платформы
func isValidVideoURL(url string) bool {
	// Базовые проверки
	if len(url) < 10 {
		return false
	}
	
	// Проверяем только YouTube платформы
	supportedPatterns := []string{
		"youtube.com", "youtu.be",           // YouTube
	}
	
	for _, pattern := range supportedPatterns {
		if strings.Contains(url, pattern) {
			return true
		}
	}
	
	return false
}

// HealthCheck проверяет состояние всех сервисов
func HealthCheck(youtubeService *services.YouTubeService, cacheService *services.CacheService) map[string]string {
	health := make(map[string]string)
	
	// Проверяем yt-dlp
	if err := youtubeService.CheckYtDlp(); err != nil {
		health["yt-dlp"] = "❌ " + err.Error()
	} else {
		health["yt-dlp"] = "✅ Работает"
	}
	
	// Проверяем сетевое подключение
	if err := youtubeService.CheckNetwork(); err != nil {
		health["network"] = "❌ " + err.Error()
	} else {
		health["network"] = "✅ Работает"
	}
	
	// Проверяем кэш-сервис
	if cacheService != nil {
		health["cache"] = "✅ Работает"
	} else {
		health["cache"] = "⚠️ Не инициализирован"
	}
	
	// Проверяем Telegram API
	health["telegram"] = "✅ Работает"
	
	return health
}

// CleanupCache очищает старые данные кэша
func CleanupCache(bot *LocalBot) {
	log.Println("🧹 Запуск очистки кэша...")
	
	// Очищаем старые данные из памяти
	clearedChats := 0
	for chatID, lastTime := range bot.lastRequestTime {
		if time.Since(lastTime) > 24*time.Hour {
			delete(bot.formatCache, chatID)
			delete(bot.videoURLCache, chatID)
			delete(bot.platformCache, chatID)
			delete(bot.lastRequestTime, chatID)
			clearedChats++
		}
	}
	
	if clearedChats > 0 {
		log.Printf("🧹 Очищено %d неактивных чатов из кэша", clearedChats)
	}
	
	log.Printf("📊 Текущий размер кэша: %d чатов, %d URL, %d платформ", 
		len(bot.formatCache), len(bot.videoURLCache), len(bot.platformCache))
}

// UpdateMetrics обновляет метрики бота
func (b *LocalBot) UpdateMetrics(requestType string, success bool, duration time.Duration) {
	b.metrics.TotalRequests++
	b.metrics.LastActivity = time.Now()
	
	if success {
		b.metrics.SuccessfulRequests++
		if requestType == "download" {
			b.metrics.TotalDownloads++
		}
	} else {
		b.metrics.FailedRequests++
		b.metrics.TotalErrors++
	}
	
	// Обновляем среднее время ответа
	if b.metrics.TotalRequests > 0 {
		totalDuration := b.metrics.AverageResponseTime * time.Duration(b.metrics.TotalRequests-1)
		b.metrics.AverageResponseTime = (totalDuration + duration) / time.Duration(b.metrics.TotalRequests)
	}
}

// GetMetrics возвращает текущие метрики бота
func (b *LocalBot) GetMetrics() *BotMetrics {
	return b.metrics
}

// GetUptime возвращает время работы бота
func (b *LocalBot) GetUptime() time.Duration {
	return time.Since(b.metrics.StartTime)
}

// IsAdmin проверяет, является ли пользователь администратором
func (b *LocalBot) IsAdmin(userID int64) bool {
	return b.adminIDs[userID]
}

// formatDuration форматирует продолжительность в читаемый вид
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0f сек", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.0f мин", d.Minutes())
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%.1f ч", d.Hours())
	} else {
		days := int(d.Hours() / 24)
		hours := int(d.Hours()) % 24
		return fmt.Sprintf("%d дн %d ч", days, hours)
	}
}

// formatTime форматирует время в читаемый вид
func formatTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)
	
	if diff < time.Minute {
		return "только что"
	} else if diff < time.Hour {
		return fmt.Sprintf("%.0f мин назад", diff.Minutes())
	} else if diff < 24*time.Hour {
		return fmt.Sprintf("%.0f ч назад", diff.Hours())
	} else {
		return t.Format("02.01.2006 15:04")
	}
}
