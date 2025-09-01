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
	// Сервис для работы с YouTube
	youtubeService *services.YouTubeService
	// Сервис для кэширования популярных видео
	cacheService *services.CacheService
}

// NewLocalBot создает новый экземпляр LocalBot
func NewLocalBot(token, apiURL string, timeout time.Duration, youtubeService *services.YouTubeService, cacheService *services.CacheService) *LocalBot {
	return &LocalBot{
		Token:  token,
		APIURL: apiURL,
		Client: &http.Client{
			Timeout: timeout,
		},
		formatCache: make(map[int64][]services.VideoFormat),
		videoURLCache: make(map[int64]string),
		youtubeService: youtubeService,
		cacheService: cacheService,
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

// SendVideo отправляет видео файл
func (b *LocalBot) SendVideo(chatID int64, videoPath, caption string) error {
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

	// Создаем сервис для кэширования (20 ГБ) - рядом с корнем проекта
	cacheService, err := services.NewCacheService("../cache", 20)
	if err != nil {
		log.Fatalf("❌ Ошибка создания кэш-сервиса: %v", err)
	}
	defer cacheService.Close()
	
	// Создаем локального бота
	bot := NewLocalBot(cfg.TelegramToken, cfg.TelegramAPI, time.Duration(cfg.HTTPTimeout)*time.Second, youtubeService, cacheService)

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
						// YouTube ссылка - показываем доступные форматы
						log.Printf("🔍 Обрабатываю YouTube ссылку: %s", message.Text)
						
						go func() {
							// Очищаем старый кэш для этого чата ВНУТРИ горутины
							delete(bot.formatCache, message.Chat.ID)
							delete(bot.videoURLCache, message.Chat.ID)
							log.Printf("🗑️ Очистил старый кэш для чата %d", message.Chat.ID)
							
							// Очищаем историю чата (удаляем старые сообщения бота)
							if err := bot.ClearChatHistory(message.Chat.ID); err != nil {
								log.Printf("⚠️ Не удалось очистить историю чата: %v", err)
							}
							
							log.Printf("🚀 Запускаю анализ форматов для: %s", message.Text)
							bot.SendMessage(message.Chat.ID, "🔍 Анализирую доступные форматы видео...")
							
							// Получаем список форматов
							log.Printf("📋 Вызываю GetVideoFormats...")
							formats, err := youtubeService.GetVideoFormats(message.Text)
							if err != nil {
								log.Printf("❌ Ошибка GetVideoFormats: %v", err)
								bot.SendMessage(message.Chat.ID, fmt.Sprintf("❌ Ошибка получения форматов: %v", err))
								return
							}
							
							log.Printf("📊 Получено форматов: %d", len(formats))
							
							// Проверяем, что URL в кэше соответствует текущему запросу
							cachedURL := bot.videoURLCache[message.Chat.ID]
							if cachedURL != "" && cachedURL != message.Text {
								log.Printf("⚠️ ВНИМАНИЕ: URL в кэше не соответствует текущему запросу!")
								log.Printf("  Кэш: %s", cachedURL)
								log.Printf("  Текущий: %s", message.Text)
								// Очищаем кэш и сохраняем новый URL
								delete(bot.formatCache, message.Chat.ID)
								delete(bot.videoURLCache, message.Chat.ID)
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
								bot.SendMessage(message.Chat.ID, "❌ Не найдено доступных форматов для скачивания.")
								return
							}
							
							// Сохраняем форматы и URL в кэше для этого чата
							bot.formatCache[message.Chat.ID] = formats
							bot.videoURLCache[message.Chat.ID] = message.Text
							log.Printf("💾 Сохранил в кэш: %d форматов и URL: %s для чата %d", len(formats), message.Text, message.Chat.ID)
							
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
							
							// Для каждого разрешения выбираем ТОЛЬКО форматы С АУДИО
							for resolution, formats := range resolutionGroups {
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
									log.Printf("🎥 Добавлен в видео: %s (%s) - %s (аудио: true)", 
										audioFormat.ID, audioFormat.Resolution, audioFormat.FileSize)
								} else {
									log.Printf("⏭️ Пропускаю разрешение %s - нет форматов с аудио", resolution)
								}
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
							
							// Отправляем подменю выбора типа
							if err := bot.SendFormatTypeMenu(message.Chat.ID, len(audioFormats), len(videoFormats)); err != nil {
								log.Printf("❌ Ошибка отправки меню выбора типа: %v", err)
								bot.SendMessage(message.Chat.ID, "❌ Ошибка создания меню выбора")
							}
							
							// НЕ скачиваем автоматически - ждем команду пользователя
							log.Printf("⏸️ Ожидаю выбор пользователя...")
						}()
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
								log.Printf("🚀 Начинаю загрузку видео в формате %s", formatID)
								bot.SendMessage(callback.Message.Chat.ID, "🔄 Начинаю загрузку...")
								
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
									// Извлекаем Video ID для проверки кэша
									videoID := extractVideoID(videoURL)
									if videoID == "" {
										log.Printf("❌ Не удалось извлечь Video ID из URL: %s", videoURL)
										bot.SendMessage(callback.Message.Chat.ID, "❌ Ошибка: неверный формат ссылки")
										return
									}
									
									// Проверяем кэш
									if isCached, cachedVideo, err := bot.cacheService.IsVideoCached(videoID, formatID); err != nil {
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
										bot.cacheService.IncrementDownloadCount(videoID, formatID)
										
										log.Printf("✅ Видео отправлено из кэша: %s", formatID)
										bot.SendMessage(callback.Message.Chat.ID, "✅ Видео отправлено из кэша!")
										return
									}
									
									// Видео не в кэше - скачиваем
									log.Printf("📥 Видео не в кэше, скачиваю: %s", videoURL)
									bot.SendMessage(callback.Message.Chat.ID, "📥 Скачиваю файл...")
									
									// Реальная загрузка через youtubeService
									videoPath, err := bot.youtubeService.DownloadVideoWithFormat(videoURL, formatID)
									if err != nil {
										log.Printf("❌ Ошибка загрузки видео: %v", err)
										bot.SendMessage(callback.Message.Chat.ID, fmt.Sprintf("❌ Ошибка загрузки: %v", err))
										return
									}
									
									log.Printf("📥 Файл скачан: %s", videoPath)
									bot.SendMessage(callback.Message.Chat.ID, "📤 Отправляю файл в Telegram...")
									
									// Определяем тип файла по расширению
									fileExt := strings.ToLower(filepath.Ext(videoPath))
									isAudio := fileExt == ".mp3" || fileExt == ".m4a" || fileExt == ".webm" || fileExt == ".ogg"
									
									// Отправляем файл в Telegram
									if isAudio {
										// Для аудио файлов
										if err := bot.SendVideo(callback.Message.Chat.ID, videoPath, fmt.Sprintf("Аудио в формате %s", formatID)); err != nil {
											log.Printf("❌ Ошибка отправки аудио: %v", err)
											bot.SendMessage(callback.Message.Chat.ID, fmt.Sprintf("❌ Ошибка отправки: %v", err))
											return
										}
										
										log.Printf("✅ Аудио успешно отправлено: %s", formatID)
										bot.SendMessage(callback.Message.Chat.ID, "✅ Аудио успешно отправлено!")
									} else {
										// Для видео файлов
										if err := bot.SendVideo(callback.Message.Chat.ID, videoPath, fmt.Sprintf("Видео в формате %s", formatID)); err != nil {
											log.Printf("❌ Ошибка отправки видео: %v", err)
											bot.SendMessage(callback.Message.Chat.ID, fmt.Sprintf("❌ Ошибка отправки: %v", err))
											return
										}
										
										log.Printf("✅ Видео успешно отправлено: %s", formatID)
										bot.SendMessage(callback.Message.Chat.ID, "✅ Видео успешно отправлено!")
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
											if err := bot.cacheService.AddToCache(videoID, videoURL, "YouTube Video", formatID, resolution, videoPath, fileInfo.Size()); err != nil {
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
							bot.SendMessage(callback.Message.Chat.ID, "🔄 Начинаю загрузку...")
							
							// TODO: Здесь нужно скачать видео в лучшем качестве
							// Пока просто логируем
							log.Printf("📥 Мгновенная загрузка завершена")
							bot.SendMessage(callback.Message.Chat.ID, "✅ Загрузка завершена!")
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
