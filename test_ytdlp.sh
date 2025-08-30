#!/bin/bash

# Тестирование yt-dlp для диагностики проблем

echo "🧪 Тестирование yt-dlp"
echo "======================"

# Проверяем наличие yt-dlp
if ! command -v yt-dlp &> /dev/null; then
    echo "❌ yt-dlp не установлен"
    echo "Установите: sudo apt install yt-dlp"
    exit 1
fi

echo "✅ yt-dlp найден: $(yt-dlp --version)"

# Тестовый URL
TEST_URL="https://www.youtube.com/watch?v=dQw4w9WgXcQ"

echo ""
echo "🔍 Тестирую получение форматов для: $TEST_URL"
echo "----------------------------------------"

# Получаем список форматов
yt-dlp --list-formats --no-playlist --no-check-certificates "$TEST_URL"

if [ $? -eq 0 ]; then
    echo ""
    echo "✅ Форматы получены успешно"
    
    echo ""
    echo "💾 Тестирую скачивание (только первые 10 секунд)..."
    echo "----------------------------------------"
    
    # Скачиваем только первые 10 секунд для теста
    yt-dlp --format "best[ext=mp4]/best" \
           --output "test_video.%(ext)s" \
           --no-playlist \
           --no-check-certificates \
           --max-duration 10 \
           "$TEST_URL"
    
    if [ $? -eq 0 ]; then
        echo ""
        echo "✅ Тестовое скачивание успешно"
        
        # Проверяем, что файл создался
        if [ -f test_video.* ]; then
            echo "📁 Создан файл: $(ls test_video.*)"
            echo "🗑️ Удаляю тестовый файл..."
            rm test_video.*
        fi
    else
        echo ""
        echo "❌ Ошибка при тестовом скачивании"
    fi
else
    echo ""
    echo "❌ Ошибка при получении форматов"
fi

echo ""
echo "🔧 Дополнительная диагностика:"
echo "=============================="

# Проверяем версию Python (yt-dlp зависит от Python)
echo "🐍 Python версия: $(python3 --version 2>/dev/null || echo 'Python не найден')"

# Проверяем доступность YouTube
echo "🌐 Проверяю доступность YouTube..."
if ping -c 1 youtube.com &> /dev/null; then
    echo "✅ YouTube доступен"
else
    echo "❌ YouTube недоступен"
fi

# Проверяем права на папку загрузок
echo "📁 Проверяю папку загрузок..."
if [ -d "./downloads" ]; then
    echo "✅ Папка downloads существует"
    echo "📝 Права: $(ls -ld ./downloads)"
else
    echo "❌ Папка downloads не существует"
    echo "🔨 Создаю папку downloads..."
    mkdir -p ./downloads
    echo "✅ Папка создана"
fi

echo ""
echo "🎯 Рекомендации:"
echo "================"
echo "1. Если yt-dlp работает, но бот не может скачать - проверьте логи бота"
echo "2. Если yt-dlp не работает - обновите его: pip install --upgrade yt-dlp"
echo "3. Проверьте, что видео публичное и доступно в вашем регионе"

