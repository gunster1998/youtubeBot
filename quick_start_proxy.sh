#!/bin/bash

# 🚀 Быстрый запуск YouTube Bot с SOCKS5 прокси
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

echo "🚀 Быстрый запуск YouTube Bot с SOCKS5 прокси"
echo "=============================================="

# Проверяем права
if [[ $EUID -eq 0 ]]; then
   error "Не запускайте этот скрипт от root! Используйте обычного пользователя."
   exit 1
fi

# Функция для проверки команды
check_command() {
    if command -v "$1" &> /dev/null; then
        log "✅ $1 установлен"
        return 0
    else
        error "❌ $1 не установлен"
        return 1
    fi
}

# Проверяем зависимости
log "🔍 Проверяю зависимости..."

missing_deps=0

if ! check_command "go"; then
    warn "Установите Go: https://golang.org/dl/"
    missing_deps=1
fi

if ! check_command "yt-dlp"; then
    warn "Установите yt-dlp: pip install yt-dlp"
    missing_deps=1
fi

if ! check_command "curl"; then
    warn "Установите curl: sudo apt install curl"
    missing_deps=1
fi

if [ $missing_deps -eq 1 ]; then
    error "Установите недостающие зависимости и запустите скрипт снова"
    exit 1
fi

# Проверяем .env файл
if [ ! -f ".env" ]; then
    if [ -f "env.example" ]; then
        log "📝 Создаю .env из env.example..."
        cp env.example .env
        log "✅ .env создан. Отредактируйте настройки прокси: nano .env"
    else
        error "Файл env.example не найден!"
        exit 1
    fi
else
    log "✅ .env найден"
fi

# Проверяем настройки прокси
log "🔍 Проверяю настройки прокси..."
source .env 2>/dev/null || true

if [ "$USE_PROXY" = "true" ]; then
    log "🌐 Прокси включен: $PROXY_URL"
    
    # Тестируем прокси
    log "🧪 Тестирую прокси соединение..."
    if curl --connect-timeout 10 --proxy "$PROXY_URL" https://www.google.com -s -o /dev/null; then
        log "✅ Прокси работает"
    else
        warn "⚠️ Прокси не работает, но продолжаем"
    fi
else
    log "ℹ️ Прокси отключен (USE_PROXY=false)"
fi

# Собираем проект
log "🔨 Собираю проект..."
if go build -o youtubeBot cmd/bot/main.go; then
    log "✅ Проект собран успешно"
else
    error "❌ Ошибка сборки проекта"
    exit 1
fi

# Проверяем Telegram Bot API
log "📱 Проверяю Telegram Bot API..."
if curl -s http://127.0.0.1:8081/health > /dev/null 2>&1; then
    log "✅ Telegram Bot API работает"
else
    warn "⚠️ Telegram Bot API не отвечает на порту 8081"
    log "💡 Убедитесь что локальный сервер Telegram API запущен"
fi

# Очищаем старые файлы
log "🧹 Очищаю старые файлы..."
rm -f last_offset.txt
rm -f bot.log

# Запускаем самопроверку
log "🧪 Запускаю самопроверку..."
if [ -f "scripts/run_selftest.sh" ]; then
    ./scripts/run_selftest.sh
else
    warn "⚠️ Скрипт самопроверки не найден"
fi

# Запускаем бота
log "🚀 Запускаю YouTube Bot..."
echo ""
log "📋 Информация:"
echo "  - Прокси: $([ "$USE_PROXY" = "true" ] && echo "$PROXY_URL" || echo "отключен")"
echo "  - Telegram API: http://127.0.0.1:8081"
echo "  - Логи: bot.log"
echo ""
log "🎬 Бот готов к работе! Отправьте ссылку на YouTube видео."
echo ""

# Запускаем бота с перенаправлением логов
exec ./youtubeBot 2>&1 | tee bot.log
