#!/bin/bash

# Тестовый скрипт для проверки аудиоформатов
# Проверяет, что аудиоформаты скачиваются в MP3, а не WebM

echo "🎵 Тестирование аудиоформатов..."
echo "=================================="

# Тестовый YouTube URL (короткое видео для быстрого тестирования)
TEST_URL="https://www.youtube.com/watch?v=dQw4w9WgXcQ"

echo "📋 Тестовый URL: $TEST_URL"
echo ""

# Проверяем доступность yt-dlp
echo "🔍 Проверяю yt-dlp..."
if ! command -v yt-dlp &> /dev/null; then
    echo "❌ yt-dlp не найден. Установите yt-dlp сначала."
    exit 1
fi

echo "✅ yt-dlp найден: $(yt-dlp --version)"
echo ""

# Создаем временную папку для тестов
TEST_DIR="/tmp/youtube_bot_audio_test"
mkdir -p "$TEST_DIR"
cd "$TEST_DIR"

echo "📁 Тестовая папка: $TEST_DIR"
echo ""

# Тест 1: Получаем список форматов
echo "🔍 Тест 1: Получение списка форматов..."
yt-dlp --list-formats --no-playlist "$TEST_URL" > formats.txt 2>&1

if [ $? -eq 0 ]; then
    echo "✅ Список форматов получен"
    
    # Ищем аудиоформаты
    echo ""
    echo "🎵 Найденные аудиоформаты:"
    grep -i "audio" formats.txt | head -5
    echo ""
    
    # Ищем WebM аудиоформаты
    echo "🔍 WebM аудиоформаты:"
    grep -i "webm.*audio\|audio.*webm" formats.txt || echo "Нет WebM аудиоформатов"
    echo ""
else
    echo "❌ Ошибка получения форматов"
    exit 1
fi

# Тест 2: Скачиваем аудио в MP3
echo "🎵 Тест 2: Скачивание аудио в MP3..."
echo "Команда: yt-dlp --extract-audio --audio-format mp3 --audio-quality 0 --output 'test_audio.%(ext)s' '$TEST_URL'"

yt-dlp --extract-audio --audio-format mp3 --audio-quality 0 --output "test_audio.%(ext)s" "$TEST_URL" > download.log 2>&1

if [ $? -eq 0 ]; then
    echo "✅ Аудио скачано"
    
    # Проверяем результат
    echo ""
    echo "📁 Скачанные файлы:"
    ls -la *.mp3 2>/dev/null || echo "MP3 файлы не найдены"
    ls -la *.webm 2>/dev/null || echo "WebM файлы не найдены"
    echo ""
    
    # Проверяем тип файла
    if [ -f "test_audio.mp3" ]; then
        echo "✅ Успех! Аудио скачано в MP3 формате"
        file test_audio.mp3
    else
        echo "❌ Ошибка! MP3 файл не создан"
        echo "Содержимое папки:"
        ls -la
    fi
else
    echo "❌ Ошибка скачивания аудио"
    echo "Лог ошибки:"
    cat download.log
fi

echo ""
echo "🧹 Очистка тестовых файлов..."
cd /
rm -rf "$TEST_DIR"

echo "✅ Тест завершен!"
