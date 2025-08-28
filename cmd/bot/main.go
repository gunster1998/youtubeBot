package main

import (
	"log"
	"net/http"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"youtubeBot/config"
	"youtubeBot/handlers"
	"youtubeBot/services"
)

func main() {
	// Загружаем конфигурацию
	cfg, err := config.Load("config.env")
	if err != nil {
		log.Printf("Предупреждение: не удалось загрузить config.env: %v", err)
	}

	// Проверяем наличие yt-dlp
	youtubeService := services.NewYouTubeService(cfg.DownloadDir)
	if err := youtubeService.CheckYtDlp(); err != nil {
		log.Fatal(err)
	}

	// Получаем токен бота из переменной окружения
	token := cfg.TelegramToken
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN не установлен")
	}

	log.Printf("Запуск бота с токеном: %s...", token[:10]+"...")

	// Создаем HTTP клиент
	client := &http.Client{
		Timeout: time.Duration(cfg.HTTPTimeout) * time.Second,
	}

	// Создаем бота
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatal(err)
	}

	// Устанавливаем кастомный HTTP клиент
	api.Client = client

	// Создаем обработчик
	handler := handlers.NewTelegramHandler(api, youtubeService)

	// Запускаем бота
	startBot(api, handler)
}

// startBot запускает бота
func startBot(api *tgbotapi.BotAPI, handler *handlers.TelegramHandler) {
	log.Printf("🚀 Бот запущен: %s", api.Self.UserName)
	log.Printf("📝 Имя бота: %s", api.Self.FirstName)
	log.Printf("🆔 ID бота: %d", api.Self.ID)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = cfg.TelegramTimeout // Используем настройку из конфига

	// Получаем канал обновлений
	updates := api.GetUpdatesChan(u)

	// Обрабатываем обновления
	for update := range updates {
		if update.Message != nil {
			log.Printf("📨 Получено сообщение от %s (%d): %s",
				update.Message.From.UserName,
				update.Message.From.ID,
				update.Message.Text)

			go handler.HandleMessage(update.Message)
		} else if update.CallbackQuery != nil {
			// Обрабатываем нажатия на кнопки
			log.Printf("🔘 Получен callback: %s", update.CallbackQuery.Data)
			go handler.HandleCallback(update.CallbackQuery)
		}
	}
}
