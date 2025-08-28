#!/bin/bash

# Скрипт для установки yt-dlp на сервере

echo "🔧 Установка yt-dlp на сервере"
echo "=============================="

# Проверяем, что мы root
if [ "$EUID" -ne 0 ]; then
    echo "❌ Этот скрипт должен запускаться от root"
    echo "Запустите: sudo ./install_ytdlp.sh"
    exit 1
fi

echo "✅ Запущено от root"

# Обновляем пакеты
echo "📦 Обновляю пакеты..."
apt update

# Устанавливаем python3-pip если не установлен
if ! command -v pip3 &> /dev/null; then
    echo "🐍 Устанавливаю python3-pip..."
    apt install -y python3-pip
else
    echo "✅ python3-pip уже установлен"
fi

# Устанавливаем yt-dlp через pip
echo "📥 Устанавливаю yt-dlp через pip..."
pip3 install --upgrade yt-dlp

# Проверяем установку
if command -v yt-dlp &> /dev/null; then
    echo "✅ yt-dlp установлен успешно"
    echo "📊 Версия: $(yt-dlp --version)"
    
    # Создаем символическую ссылку в /usr/local/bin если нужно
    if [ ! -f /usr/local/bin/yt-dlp ]; then
        echo "🔗 Создаю символическую ссылку..."
        ln -sf $(which yt-dlp) /usr/local/bin/yt-dlp
    fi
    
    echo ""
    echo "🎉 yt-dlp готов к использованию!"
    echo "🚀 Теперь можно запускать бота"
    
else
    echo "❌ Ошибка установки yt-dlp"
    exit 1
fi

echo ""
echo "🔧 Дополнительные настройки:"
echo "============================"

# Проверяем права на папку загрузок
echo "📁 Проверяю папку загрузок..."
if [ ! -d "./downloads" ]; then
    echo "🔨 Создаю папку downloads..."
    mkdir -p ./downloads
    chmod 755 ./downloads
fi

echo "✅ Готово! Теперь запустите бота:"
echo "go run cmd/bot/main.go"

