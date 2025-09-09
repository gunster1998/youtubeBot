#!/bin/bash

echo "🔄 Обновление YouTube Bot сервиса..."

# Останавливаем сервис
echo "⏹️ Останавливаем сервис..."
sudo systemctl stop youtubebot

# Переходим в директорию проекта
cd /home/gunster1998/repos/youtubeBot

# Обновляем код
echo "📥 Обновляем код из GitHub..."
git pull origin main

# Собираем проект
echo "🔨 Собираем проект..."
go mod tidy
go build -o youtubeBot cmd/bot/main.go

# Проверяем что сборка прошла успешно
if [ $? -eq 0 ]; then
    echo "✅ Сборка успешна"
else
    echo "❌ Ошибка сборки!"
    exit 1
fi

# Запускаем сервис
echo "🚀 Запускаем сервис..."
sudo systemctl start youtubebot

# Проверяем статус
echo "📊 Статус после обновления:"
sudo systemctl status youtubebot --no-pager

echo ""
echo "✅ Обновление завершено!"
echo "📝 Для просмотра логов: sudo journalctl -u youtubebot -f"
