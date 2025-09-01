package handlers

import (
	"fmt"
	"log"
	"strings"
	"time"

	"youtubeBot/services"
	"youtubeBot/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// min возвращает минимальное из двух чисел
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TelegramHandler обрабатывает сообщения Telegram
type TelegramHandler struct {
	api            *tgbotapi.BotAPI
	youtubeService *services.YouTubeService
}

// NewTelegramHandler создает новый обработчик Telegram
func NewTelegramHandler(api *tgbotapi.BotAPI, youtubeService *services.YouTubeService) *TelegramHandler {
	return &TelegramHandler{
		api:            api,
		youtubeService: youtubeService,
	}
}

// HandleMessage обрабатывает входящие сообщения
func (h *TelegramHandler) HandleMessage(message *tgbotapi.Message) {
	log.Printf("📨 Получено сообщение: %s от пользователя %s (ID: %d)", 
		message.Text, message.From.UserName, message.From.ID)
	
	if !message.IsCommand() {
		log.Printf("❌ Сообщение не является командой: %s", message.Text)
		return
	}

	log.Printf("✅ Обрабатываю команду: %s", message.Command())

	switch message.Command() {
	case "start":
		log.Printf("🚀 Обрабатываю команду /start")
		h.sendWelcomeMessage(message.Chat.ID)
	case "help":
		log.Printf("📚 Обрабатываю команду /help")
		h.sendHelpMessage(message.Chat.ID)
	case "download":
		log.Printf("📥 Обрабатываю команду /download")
		args := message.CommandArguments()
		if args == "" {
			h.sendMessage(message.Chat.ID, "Пожалуйста, укажите URL YouTube видео после команды /download")
			return
		}
		h.handleDownload(message.Chat.ID, args)
	default:
		log.Printf("❓ Неизвестная команда: %s", message.Command())
		h.sendMessage(message.Chat.ID, "Неизвестная команда. Используйте /help для получения справки.")
	}
}

// HandleCallback обрабатывает нажатия на кнопки
func (h *TelegramHandler) HandleCallback(callback *tgbotapi.CallbackQuery) {
	// Парсим callback data
	data := strings.Split(callback.Data, ":")
	
	if len(data) == 3 && data[0] == "download" {
		// Обычное скачивание в выбранном формате
		videoID := data[1]
		formatID := data[2]

		// Отправляем сообщение о начале загрузки
		msg := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID,
			fmt.Sprintf("🔄 Скачиваю видео в формате %s...", formatID))
		h.api.Send(msg)

		// Скачиваем видео в выбранном формате
		videoPath, err := h.youtubeService.DownloadVideoWithFormat(videoID, formatID)
		if err != nil {
			h.sendMessage(callback.Message.Chat.ID, fmt.Sprintf("❌ Ошибка при скачивании: %v", err))
			return
		}

		// Отправляем видео
		h.sendVideo(callback.Message.Chat.ID, videoPath)
		
	} else if len(data) == 2 && data[0] == "quick" {
		// Быстрое скачивание
		videoID := data[1]
		url := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)

		// Отправляем сообщение о начале быстрой загрузки
		msg := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID,
			"⚡ Быстрое скачивание в лучшем качестве...")
		h.api.Send(msg)

		// Быстро скачиваем видео
		videoPath, err := h.youtubeService.DownloadVideoFast(url)
		if err != nil {
			h.sendMessage(callback.Message.Chat.ID, fmt.Sprintf("❌ Ошибка при быстром скачивании: %v", err))
			return
		}

		// Отправляем видео
		h.sendVideo(callback.Message.Chat.ID, videoPath)
	}
}

// sendWelcomeMessage отправляет приветственное сообщение
func (h *TelegramHandler) sendWelcomeMessage(chatID int64) {
	log.Printf("📨 Отправляю приветственное сообщение в чат %d", chatID)
	
	text := `Привет✊

@TubeLoaderBot - бот для скачивания видео и аудио.

Поддерживаются сайты:

 🚀 YouTube
 🚀 VK
 🚀 RuTube
 🚀 Yandex.Dzen
 
 Зачем?
 
 🌀 Доступ к видео без блокировок, VPN и других сложностей
 🌀 Скачай видео заранее, и просматривай потом, даже когда не будет доступа к интернету
 🌀 Фоновый режим: можно просматривать видео, даже когда используешь другие приложения
 🌀 Бесплатно

🔽 Как это работает? 🔽

📋 Доступные команды:
/download <URL> - Скачать видео с выбором качества
/help - Показать справку

⚠️ Используйте бота только для скачивания видео, на которые у вас есть права.`

	h.sendMessage(chatID, text)
}

// sendHelpMessage отправляет справку
func (h *TelegramHandler) sendHelpMessage(chatID int64) {
	text := `📚 Справка по использованию бота:

🔗 Скачать видео:
/download https://www.youtube.com/watch?v=VIDEO_ID

📝 Пример:
/download https://www.youtube.com/watch?v=dQw4w9WgXcQ

⚠️ Важно:
• Поддерживаются только публичные видео
• Соблюдайте авторские права
• Используйте только для личного использования`

	h.sendMessage(chatID, text)
}

// handleDownload обрабатывает команду скачивания
func (h *TelegramHandler) handleDownload(chatID int64, url string) {
	// Отправляем сообщение о начале анализа
	msg := tgbotapi.NewMessage(chatID, "🔍 Анализирую доступные форматы видео...")
	h.api.Send(msg)

	// Получаем доступные форматы
	formats, err := h.youtubeService.GetVideoFormats(url)
	if err != nil {
		// Предлагаем быстрое скачивание при ошибке анализа
		errorMsg := fmt.Sprintf("❌ Ошибка при получении форматов: %s\n\n💡 Попробуйте быстрое скачивание в лучшем качестве:", err.Error())
		
		// Добавляем диагностическую информацию
		if strings.Contains(err.Error(), "таймаут") {
			errorMsg += "\n\n⚠️ Возможные причины:\n• Медленный интернет\n• Блокировка YouTube\n• Проблемы с сетью"
		} else if strings.Contains(err.Error(), "SSL") || strings.Contains(err.Error(), "handshake") {
			errorMsg += "\n\n⚠️ Возможные причины:\n• Проблемы с SSL сертификатами\n• Блокировка на уровне провайдера\n• Необходимость VPN/прокси"
		}
		
		h.sendMessage(chatID, errorMsg)
		h.showQuickDownloadOption(chatID, url)
		return
	}

	// Показываем выбор форматов
	h.showFormatSelection(chatID, url, formats)
}

// showFormatSelection показывает кнопки для выбора формата
func (h *TelegramHandler) showFormatSelection(chatID int64, url string, formats []services.VideoFormat) {
	if len(formats) == 0 {
		h.sendMessage(chatID, "❌ Не удалось получить доступные форматы видео")
		return
	}

	// Создаем кнопки для выбора формата
	var buttons [][]tgbotapi.InlineKeyboardButton

	// Показываем первые 10 форматов для простоты
	maxFormats := 10
	if len(formats) < maxFormats {
		maxFormats = len(formats)
	}

	for i := 0; i < maxFormats; i++ {
		format := formats[i]

		// Создаем текст кнопки
		buttonText := fmt.Sprintf("📹 %s %s", format.Resolution, format.Extension)
		if format.HasAudio {
			buttonText += " 🔊"
		} else {
			buttonText += " 🔇"
		}

		if format.FileSize != "" {
			buttonText += fmt.Sprintf(" (%s)", format.FileSize)
		}

		// Создаем callback data
		callbackData := fmt.Sprintf("download:%s:%s", utils.ExtractVideoID(url), format.ID)

		button := tgbotapi.NewInlineKeyboardButtonData(buttonText, callbackData)
		buttons = append(buttons, []tgbotapi.InlineKeyboardButton{button})
	}

	// Добавляем кнопку отмены
	cancelButton := tgbotapi.NewInlineKeyboardButtonData("❌ Отмена", "cancel")
	buttons = append(buttons, []tgbotapi.InlineKeyboardButton{cancelButton})

	// Создаем клавиатуру
	keyboard := tgbotapi.NewInlineKeyboardMarkup(buttons...)

	// Отправляем сообщение с выбором
	msg := tgbotapi.NewMessage(chatID, "📹 Выберите качество видео:")
	msg.ReplyMarkup = keyboard
	h.api.Send(msg)
}

// sendVideo отправляет видео в чат
func (h *TelegramHandler) sendVideo(chatID int64, videoPath string) {
	// Добавляем задержку 5 секунд перед отправкой
	log.Printf("⏳ Ожидание 5 секунд перед отправкой видео...")
	time.Sleep(5 * time.Second)

	log.Printf("📤 Отправка видео в Telegram...")

	// Отправляем видео
	video := tgbotapi.NewVideo(chatID, tgbotapi.FilePath(videoPath))
	video.Caption = "✅ Видео успешно скачано!"

	_, err := h.api.Send(video)
	if err != nil {
		h.sendMessage(chatID, fmt.Sprintf("❌ Ошибка при отправке видео: %v", err))
		return
	}

	log.Printf("✅ Видео успешно отправлено в Telegram")
	log.Printf("💾 Файл сохранен в: %s", videoPath)
}

// showQuickDownloadOption показывает опцию быстрого скачивания
func (h *TelegramHandler) showQuickDownloadOption(chatID int64, url string) {
	// Создаем кнопку для быстрого скачивания
	button := tgbotapi.NewInlineKeyboardButtonData("⚡ Быстрое скачивание (лучшее качество)", 
		fmt.Sprintf("quick:%s", utils.ExtractVideoID(url)))
	
	// Создаем правильную структуру кнопок
	buttons := [][]tgbotapi.InlineKeyboardButton{{button}}
	keyboard := tgbotapi.NewInlineKeyboardMarkup(buttons...)
	
	msg := tgbotapi.NewMessage(chatID, "🎬 Выберите действие:")
	msg.ReplyMarkup = keyboard
	h.api.Send(msg)
}

// sendMessage отправляет текстовое сообщение
func (h *TelegramHandler) sendMessage(chatID int64, text string) {
	log.Printf("📤 Отправляю сообщение в чат %d: %s", chatID, text[:min(len(text), 100)])
	
	msg := tgbotapi.NewMessage(chatID, text)
	// Убираем HTML ParseMode, так как в тексте нет HTML-разметки
	// msg.ParseMode = "HTML"
	
	_, err := h.api.Send(msg)
	if err != nil {
		log.Printf("❌ Ошибка при отправке сообщения: %v", err)
	} else {
		log.Printf("✅ Сообщение успешно отправлено в чат %d", chatID)
	}
}
