package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"youtubeBot/config"
	"youtubeBot/handlers"
	"youtubeBot/services"
)

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

	// Создаем бота с локальным API
	bot, err := tgbotapi.NewBotAPIWithClient(
		cfg.TelegramToken,
		cfg.TelegramAPI,
		nil, // используем дефолтный HTTP клиент
	)
	if err != nil {
		log.Fatalf("❌ Ошибка создания бота: %v", err)
	}

	// Проверяем подключение к локальному серверу
	botInfo, err := bot.GetMe()
	if err != nil {
		log.Fatalf("❌ Не удалось подключиться к локальному серверу Telegram API: %v", err)
	}

	fmt.Printf("✅ Бот успешно подключен: @%s (%s)\n", botInfo.UserName, botInfo.FirstName)
	fmt.Printf("🌐 Используется локальный сервер: %s\n", cfg.TelegramAPI)

	// Проверяем yt-dlp
	youtubeService := services.NewYouTubeService(cfg.DownloadDir)
	if err := youtubeService.CheckYtDlp(); err != nil {
		log.Fatalf("❌ %v", err)
	}
	fmt.Println("✅ yt-dlp доступен")

	// Создаем обработчик
	handler := handlers.NewTelegramHandler(bot, youtubeService)

	// Настраиваем обновления
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60

	updates := bot.GetUpdatesChan(updateConfig)

	fmt.Println("🎬 Бот готов к работе! Отправьте ссылку на YouTube видео.")

	// Обрабатываем сигналы для graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Основной цикл обработки сообщений
	for {
		select {
		case update := <-updates:
			if update.Message != nil {
				// Обрабатываем входящие сообщения
				handler.HandleMessage(update.Message)
			} else if update.CallbackQuery != nil {
				// Обрабатываем callback'и от inline кнопок
				handler.HandleCallback(update.CallbackQuery)
				
				// Отвечаем на callback
				callback := tgbotapi.NewCallback(update.CallbackQuery.ID, "")
				bot.Send(callback)
			}

		case sig := <-sigChan:
			fmt.Printf("\n🛑 Получен сигнал %v, завершаю работу...\n", sig)
			return
		}
	}
}
