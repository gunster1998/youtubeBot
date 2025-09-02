#!/bin/bash

echo "🚀 Быстрый запуск асинхронного YouTube Bot"
echo "=========================================="

# Переходим в папку проекта
cd /home/gunster1998/repos/youtubeBotv2

echo "🧹 Очистка старых процессов..."
# Убиваем старые процессы
sudo pkill -f youtubeBot 2>/dev/null || true
sudo pkill -f "go run" 2>/dev/null || true
sudo pkill -f main.go 2>/dev/null || true

# Очищаем Docker
docker stop $(docker ps -aq) 2>/dev/null || true
docker rm $(docker ps -aq) 2>/dev/null || true

# Освобождаем порты
sudo fuser -k 8081/tcp 2>/dev/null || true
sudo fuser -k 8080/tcp 2>/dev/null || true

echo "✅ Очистка завершена"

# Создаем папки
echo "📁 Создание папок..."
mkdir -p downloads ../cache logs

# Даем права
chmod +x run_async.sh run.sh install_ytdlp.sh

# Проверяем зависимости
echo "📦 Проверка зависимостей..."
go mod tidy
go mod download

echo "🎬 Запуск асинхронного бота..."
./run_async.sh

