#!/bin/bash

# Тестовый скрипт для проверки YouTube Downloader Telegram Bot

echo "🧪 Тестирование YouTube Downloader Telegram Bot"
echo "=============================================="

# Проверяем наличие Go
if ! command -v go &> /dev/null; then
    echo "❌ Go не установлен"
    exit 1
fi

echo "✅ Go найден: $(go version)"

# Проверяем наличие yt-dlp
if ! command -v yt-dlp &> /dev/null; then
    echo "❌ yt-dlp не установлен"
    echo "Установите: sudo apt install yt-dlp или pip install yt-dlp"
    exit 1
fi

echo "✅ yt-dlp найден: $(yt-dlp --version)"

# Проверяем зависимости Go
echo "📦 Проверяю зависимости Go..."
go mod tidy

# Проверяем наличие токена
if [ -z "$TELEGRAM_BOT_TOKEN" ]; then
    echo "❌ TELEGRAM_BOT_TOKEN не установлен"
    echo ""
    echo "Установите токен:"
    echo "export TELEGRAM_BOT_TOKEN='your_token_here'"
    echo ""
    echo "Или создайте config.env с содержимым:"
    echo "TELEGRAM_BOT_TOKEN=your_token_here"
    exit 1
fi

echo "✅ Токен найден: ${TELEGRAM_BOT_TOKEN:0:10}..."

# Собираем бота
echo "🔨 Собираю бота..."
go build -o youtubeBot cmd/bot/main.go

if [ $? -ne 0 ]; then
    echo "❌ Ошибка сборки"
    exit 1
fi

echo "✅ Бот собран успешно"

# Запускаем бота в фоне для тестирования
echo "🚀 Запускаю бота для тестирования..."
./youtubeBot &
BOT_PID=$!

# Ждем немного для инициализации
sleep 3

# Проверяем, что бот запущен
if ps -p $BOT_PID > /dev/null; then
    echo "✅ Бот запущен успешно (PID: $BOT_PID)"
    echo ""
    echo "📱 Теперь протестируйте бота в Telegram:"
    echo "1. Найдите вашего бота"
    echo "2. Отправьте /start"
    echo "3. Проверьте, что приходит приветственное сообщение"
    echo ""
    echo "Для остановки бота выполните: kill $BOT_PID"
else
    echo "❌ Бот не запустился"
    exit 1
fi

