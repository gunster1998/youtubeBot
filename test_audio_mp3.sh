#!/bin/bash

# Тест для проверки что аудио скачивается в MP3 формате
# Этот скрипт проверяет что все аудио форматы конвертируются в MP3

echo "🎵 Тестирование конвертации аудио в MP3 формат"
echo "=============================================="

# Создаем тестовую директорию
TEST_DIR="/tmp/youtube_bot_audio_mp3_test"
mkdir -p "$TEST_DIR"

# Тестовый YouTube URL (короткое видео для быстрого теста)
TEST_URL="https://www.youtube.com/watch?v=dQw4w9WgXcQ"

echo "📁 Тестовая директория: $TEST_DIR"
echo "🔗 Тестовый URL: $TEST_URL"

# Очищаем тестовую директорию
rm -f "$TEST_DIR"/*

echo ""
echo "🔍 Шаг 1: Получение списка форматов"
echo "-----------------------------------"

# Получаем список форматов
yt-dlp --list-formats "$TEST_URL" > "$TEST_DIR/formats.txt" 2>&1

if [ $? -eq 0 ]; then
    echo "✅ Список форматов получен"
    
    # Показываем аудио форматы
    echo ""
    echo "🎵 Найденные аудио форматы:"
    grep -i "audio" "$TEST_DIR/formats.txt" | head -5
    
    # Ищем WebM аудиоформаты
    echo ""
    echo "🔍 WebM аудиоформаты (должны конвертироваться в MP3):"
    grep -i "webm.*audio" "$TEST_DIR/formats.txt" | head -3
    
else
    echo "❌ Ошибка получения форматов"
    exit 1
fi

echo ""
echo "🎵 Шаг 2: Тестирование скачивания аудио"
echo "---------------------------------------"

# Скачиваем первый аудио формат
AUDIO_FORMAT=$(grep -i "audio" "$TEST_DIR/formats.txt" | head -1 | awk '{print $1}')

if [ -n "$AUDIO_FORMAT" ]; then
    echo "📥 Скачиваю аудио формат: $AUDIO_FORMAT"
    
    # Скачиваем с принудительной конвертацией в MP3
    yt-dlp \
        --format "$AUDIO_FORMAT" \
        --extract-audio \
        --audio-format mp3 \
        --audio-quality 0 \
        --output "$TEST_DIR/%(id)s_%(format_id)s.%(ext)s" \
        "$TEST_URL"
    
    if [ $? -eq 0 ]; then
        echo "✅ Аудио скачано успешно"
        
        # Проверяем что файл действительно MP3
        echo ""
        echo "🔍 Проверка формата скачанного файла:"
        ls -la "$TEST_DIR"/*.mp3 2>/dev/null
        
        if ls "$TEST_DIR"/*.mp3 >/dev/null 2>&1; then
            echo "✅ УСПЕХ: Аудио файл скачан в формате MP3!"
            
            # Проверяем что нет WebM файлов
            if ls "$TEST_DIR"/*.webm >/dev/null 2>&1; then
                echo "⚠️  ВНИМАНИЕ: Найдены WebM файлы (не должны быть):"
                ls -la "$TEST_DIR"/*.webm
            else
                echo "✅ WebM файлы не найдены - конвертация работает!"
            fi
            
        else
            echo "❌ ОШИБКА: MP3 файл не найден!"
            echo "📁 Файлы в директории:"
            ls -la "$TEST_DIR"/
        fi
        
    else
        echo "❌ Ошибка скачивания аудио"
    fi
else
    echo "❌ Аудио форматы не найдены"
fi

echo ""
echo "🧹 Очистка тестовой директории"
rm -rf "$TEST_DIR"

echo ""
echo "🎵 Тест завершен!"




