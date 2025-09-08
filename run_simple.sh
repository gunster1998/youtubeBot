#!/bin/bash

# 🎬 YouTube Bot - Упрощенный запуск
# Запускает бота с обычным Telegram API (без локального сервера)

set -e

echo "🚀 YouTube Bot - Упрощенный запуск..."

# Проверяем наличие config.env
if [ ! -f "config.env" ]; then
    echo "❌ config.env не найден!"
    echo "📋 Скопируйте config.env.example в config.env и настройте его:"
    echo "   cp config.env.example config.env"
    echo "   nano config.env"
    exit 1
fi

# Создаем папку для загрузок
mkdir -p downloads

# Загружаем переменные окружения
source config.env

# Изменяем API URL на обычный Telegram API
export TELEGRAM_API_URL="https://api.telegram.org"

echo "✅ Конфигурация загружена"
echo "🤖 Токен бота: ${TELEGRAM_BOT_TOKEN:0:10}..."
echo "🌐 API URL: ${TELEGRAM_API_URL}"

# Собираем проект
echo "🔨 Сборка проекта..."
go mod tidy
go build -o youtubeBot cmd/bot/main.go

if [ $? -ne 0 ]; then
    echo "❌ Ошибка сборки проекта"
    exit 1
fi

echo "✅ Проект собран успешно!"

# Запускаем бота
echo "🎬 Запуск бота..."
echo "📱 Используется обычный Telegram API"
echo "🎬 Бот готов к работе!"
echo ""

./youtubeBot


