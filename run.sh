#!/bin/bash

# Скрипт для запуска YouTube Downloader Telegram Bot

echo "🎬 YouTube Downloader Telegram Bot"
echo "=================================="

# Проверяем наличие токена
if [ -z "$TELEGRAM_BOT_TOKEN" ]; then
    echo "❌ Ошибка: TELEGRAM_BOT_TOKEN не установлен"
    echo ""
    echo "Установите токен одним из способов:"
    echo "1. export TELEGRAM_BOT_TOKEN='your_token_here'"
    echo "2. Создайте файл config.env и добавьте TELEGRAM_BOT_TOKEN=your_token_here"
    echo "3. Запустите: source config.env && ./run.sh"
    echo ""
    echo "Как получить токен:"
    echo "1. Найдите @BotFather в Telegram"
    echo "2. Отправьте /newbot"
    echo "3. Следуйте инструкциям"
    exit 1
fi

echo "✅ Токен найден"
echo "🚀 Запускаю бота..."

# Запускаем бота
./youtubeBot
