#!/bin/bash

# 🎬 YouTube Bot - Скрипт запуска
# Автоматически собирает и запускает бота

set -e  # Останавливаемся при ошибке

echo "🚀 YouTube Bot - Запуск..."

# Проверяем наличие Go
if ! command -v go &> /dev/null; then
    echo "❌ Go не установлен. Установите: brew install go"
    exit 1
fi

# Проверяем наличие yt-dlp
if ! command -v yt-dlp &> /dev/null; then
    echo "❌ yt-dlp не установлен. Установите: brew install yt-dlp"
    exit 1
fi

# Проверяем наличие config.env
if [ ! -f "config.env" ]; then
    echo "⚠️  config.env не найден. Копирую из примера..."
    if [ -f "config.env.example" ]; then
        cp config.env.example config.env
        echo "✅ config.env создан из примера. Отредактируйте его и добавьте токен бота!"
        echo "📝 nano config.env"
        exit 1
    else
        echo "❌ config.env.example не найден!"
        exit 1
    fi
fi

# Проверяем работу локального сервера Telegram API
echo "🌐 Проверяю локальный сервер Telegram API..."
if ! curl -s http://127.0.0.1:8081/health &> /dev/null; then
    echo "⚠️  Локальный сервер Telegram API не отвечает на http://127.0.0.1:8081/health"
    echo "💡 Убедитесь что сервер запущен перед запуском бота"
    echo "🔄 Повторить проверку? (y/n)"
    read -r response
    if [[ "$response" =~ ^[Yy]$ ]]; then
        echo "🔄 Повторная проверка..."
        if ! curl -s http://127.0.0.1:8081/health &> /dev/null; then
            echo "❌ Сервер все еще не отвечает. Запустите сервер и попробуйте снова."
            exit 1
        fi
    else
        echo "❌ Запуск отменен."
        exit 1
    fi
fi

echo "✅ Локальный сервер Telegram API работает!"

# Создаем папку для загрузок
mkdir -p downloads

# Собираем проект
echo "🔨 Сборка проекта..."
if ! go build -o youtubeBot cmd/bot/main.go; then
    echo "❌ Ошибка сборки!"
    exit 1
fi

echo "✅ Проект собран успешно!"

# Загружаем переменные окружения и запускаем
echo "🚀 Запуск бота..."
echo "📱 Используется локальный сервер: http://127.0.0.1:8081"
echo "🎬 Бот готов к работе!"
echo ""

# Запускаем бота
source config.env && ./youtubeBot
