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
	if !message.IsCommand() {
		return
	}

	switch message.Command() {
	case "start":
		h.sendWelcomeMessage(message.Chat.ID)
	case "help":
		h.sendHelpMessage(message.Chat.ID)
	case "download":
		args := message.CommandArguments()
		if args == "" {
			h.sendMessage(message.Chat.ID, "Пожалуйста, укажите URL YouTube видео после команды /download")
			return
		}
		h.handleDownload(message.Chat.ID, args)
	default:
		h.sendMessage(message.Chat.ID, "Неизвестная команда. Используйте /help для получения справки.")
	}
}

// HandleCallback обрабатывает нажатия на кнопки
func (h *TelegramHandler) HandleCallback(callback *tgbotapi.CallbackQuery) {
	// Парсим callback data: "download:videoID:formatID"
	data := strings.Split(callback.Data, ":")
	if len(data) != 3 || data[0] != "download" {
		return
	}

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
}

// sendWelcomeMessage отправляет приветственное сообщение
func (h *TelegramHandler) sendWelcomeMessage(chatID int64) {
	text := `🎉 Добро пожаловать в YouTube Downloader Bot!

Этот бот поможет вам скачать видео с YouTube с выбором качества.

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
		h.sendMessage(chatID, fmt.Sprintf("❌ Ошибка при получении форматов: %v", err))
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

// sendMessage отправляет текстовое сообщение
func (h *TelegramHandler) sendMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	h.api.Send(msg)
}
