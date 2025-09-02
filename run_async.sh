#!/bin/bash

# Скрипт запуска асинхронной версии YouTube Bot

echo "🚀 Запуск асинхронной версии YouTube Bot..."

# Проверяем наличие config.env
if [ ! -f "config.env" ]; then
    echo "❌ Файл config.env не найден!"
    echo "📋 Скопируйте config.env.example в config.env и настройте его:"
    echo "   cp config.env.example config.env"
    echo "   nano config.env"
    exit 1
fi

# Проверяем наличие yt-dlp
if ! command -v yt-dlp &> /dev/null; then
    echo "❌ yt-dlp не найден в системе!"
    echo "📋 Установите yt-dlp:"
    echo "   ./install_ytdlp.sh"
    exit 1
fi

# Создаем папку для загрузок
mkdir -p downloads

# Создаем папку для кэша
mkdir -p ../cache

# Загружаем переменные окружения
source config.env

# Проверяем обязательные переменные
if [ -z "$TELEGRAM_BOT_TOKEN" ]; then
    echo "❌ TELEGRAM_BOT_TOKEN не установлен в config.env"
    exit 1
fi

echo "✅ Конфигурация загружена"
echo "🤖 Токен бота: ${TELEGRAM_BOT_TOKEN:0:10}..."
echo "🌐 API URL: ${TELEGRAM_API_URL:-http://127.0.0.1:8081}"

# Собираем проект
echo "🔨 Сборка проекта..."

# Проверяем и исправляем зависимости
echo "📦 Проверка зависимостей..."
go mod tidy
go mod download

# Собираем проект
go build -o youtubeBot_async cmd/bot/main_async.go

if [ $? -ne 0 ]; then
    echo "❌ Ошибка сборки проекта"
    echo "🔧 Попробуйте исправить зависимости:"
    echo "   go get github.com/mattn/go-sqlite3"
    echo "   go mod tidy"
    exit 1
fi

echo "✅ Проект собран успешно"

# Запускаем бота
echo "🎬 Запуск асинхронного бота..."
echo "📊 Очередь загрузок: 3 воркера"
echo "💾 Кэш: 20 ГБ"
echo "🔄 Асинхронная обработка: включена"
echo "🔧 Исправления: кэшированные видео теперь отправляются пользователям"
echo ""
echo "🛑 Для остановки нажмите Ctrl+C"
echo ""

# Проверяем что локальный сервер Telegram API работает
if ! curl -s http://127.0.0.1:8081/health > /dev/null 2>&1; then
    echo "⚠️ Локальный сервер Telegram API не отвечает на порту 8081"
    echo "🔧 Убедитесь что telegram-bot-api запущен:"
    echo "   nohup /usr/local/bin/telegram-bot-api --api-id=27638369 --api-hash=0d3bea3f12b4bf0bce53fc6f19cccd60 --local --http-port=8081 --http-ip-address=127.0.0.1 > telegram-api.log 2>&1 &"
    echo ""
fi

./youtubeBot_async
