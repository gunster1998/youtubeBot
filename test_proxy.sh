#!/bin/bash

echo "🔍 Тестирование прокси для YouTube Bot"
echo "======================================"

# Проверяем переменные окружения
echo ""
echo "📋 Переменные окружения прокси:"
echo "ALL_PROXY: ${ALL_PROXY:-не установлен}"
echo "HTTP_PROXY: ${HTTP_PROXY:-не установлен}"
echo "HTTPS_PROXY: ${HTTPS_PROXY:-не установлен}"
echo "SOCKS_PROXY: ${SOCKS_PROXY:-не установлен}"
echo "NO_PROXY: ${NO_PROXY:-не установлен}"

# Проверяем доступность YouTube через разные методы
echo ""
echo "🌐 Тестирование подключения к YouTube:"

# Без прокси
echo "1️⃣ Без прокси:"
if curl -s --connect-timeout 10 --max-time 30 https://www.youtube.com > /dev/null 2>&1; then
    echo "   ✅ YouTube доступен без прокси"
else
    echo "   ❌ YouTube недоступен без прокси (ожидаемо для России)"
fi

# Через ALL_PROXY
if [ -n "$ALL_PROXY" ]; then
    echo ""
    echo "2️⃣ Через ALL_PROXY ($ALL_PROXY):"
    if curl -s --connect-timeout 10 --max-time 30 --proxy "$ALL_PROXY" https://www.youtube.com > /dev/null 2>&1; then
        echo "   ✅ YouTube доступен через ALL_PROXY"
    else
        echo "   ❌ YouTube недоступен через ALL_PROXY"
    fi
fi

# Через HTTP_PROXY
if [ -n "$HTTP_PROXY" ]; then
    echo ""
    echo "3️⃣ Через HTTP_PROXY ($HTTP_PROXY):"
    if curl -s --connect-timeout 10 --max-time 30 --proxy "$HTTP_PROXY" https://www.youtube.com > /dev/null 2>&1; then
        echo "   ✅ YouTube доступен через HTTP_PROXY"
    else
        echo "   ❌ YouTube недоступен через HTTP_PROXY"
    fi
fi

# Через HTTPS_PROXY
if [ -n "$HTTPS_PROXY" ]; then
    echo ""
    echo "4️⃣ Через HTTPS_PROXY ($HTTPS_PROXY):"
    if curl -s --connect-timeout 10 --max-time 30 --proxy "$HTTPS_PROXY" https://www.youtube.com > /dev/null 2>&1; then
        echo "   ✅ YouTube доступен через HTTPS_PROXY"
    else
        echo "   ❌ YouTube недоступен через HTTPS_PROXY"
    fi
fi

# Через SOCKS_PROXY
if [ -n "$SOCKS_PROXY" ]; then
    echo ""
    echo "5️⃣ Через SOCKS_PROXY ($SOCKS_PROXY):"
    if curl -s --connect-timeout 10 --max-time 30 --proxy "$SOCKS_PROXY" https://www.youtube.com > /dev/null 2>&1; then
        echo "   ✅ YouTube доступен через SOCKS_PROXY"
    else
        echo "   ❌ YouTube недоступен через SOCKS_PROXY"
    fi
fi

echo ""
echo "🎯 Рекомендации:"
echo "- Если YouTube недоступен без прокси - это нормально для России"
echo "- Убедитесь что хотя бы один прокси работает"
echo "- Для VLESS-Reality используйте порты 10808 (SOCKS5) и 10809 (HTTP)"
echo "- Перезапустите бота после изменения настроек прокси"


