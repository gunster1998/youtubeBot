#!/bin/bash

# 🧪 Скрипт запуска самопроверки YouTube Bot
# Автор: AI Assistant
# Версия: 1.0

set -e

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Функция для вывода сообщений
log() {
    echo -e "${GREEN}[$(date +'%Y-%m-%d %H:%M:%S')] $1${NC}"
}

warn() {
    echo -e "${YELLOW}[$(date +'%Y-%m-%d %H:%M:%S')] WARNING: $1${NC}"
}

error() {
    echo -e "${RED}[$(date +'%Y-%m-%d %H:%M:%S')] ERROR: $1${NC}"
}

echo "🧪 Запуск самопроверки YouTube Bot с прокси"
echo "============================================"

# Проверяем наличие config.env
if [ ! -f "config.env" ]; then
    error "Файл config.env не найден!"
    echo "Создайте config.env на основе env.example"
    exit 1
fi

# Проверяем зависимости
log "🔍 Проверяю зависимости..."

missing_deps=0

if ! command -v go &> /dev/null; then
    error "Go не установлен"
    missing_deps=1
fi

if ! command -v yt-dlp &> /dev/null; then
    error "yt-dlp не установлен"
    missing_deps=1
fi

if ! command -v curl &> /dev/null; then
    error "curl не установлен"
    missing_deps=1
fi

if [ $missing_deps -eq 1 ]; then
    error "Установите недостающие зависимости и запустите скрипт снова"
    exit 1
fi

log "✅ Все зависимости установлены"

# Компилируем и запускаем самопроверку
log "🔨 Компилирую самопроверку..."
if go build -o selftest scripts/selftest.go; then
    log "✅ Самопроверка скомпилирована"
else
    error "❌ Ошибка компиляции самопроверки"
    exit 1
fi

# Запускаем самопроверку
log "🚀 Запускаю самопроверку..."
./selftest

# Очищаем временные файлы
rm -f selftest

log "🎉 Самопроверка завершена!"

