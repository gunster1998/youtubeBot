#!/bin/bash

echo "📊 Мониторинг YouTube Bot сервиса"
echo "=================================="

# Проверяем статус сервиса
echo "🔍 Статус сервиса:"
sudo systemctl is-active youtubebot
echo ""

# Показываем последние логи
echo "📝 Последние логи (20 строк):"
sudo journalctl -u youtubebot -n 20 --no-pager
echo ""

# Показываем использование ресурсов
echo "💾 Использование ресурсов:"
ps aux | grep youtubebot | grep -v grep
echo ""

# Показываем порты
echo "🌐 Открытые порты:"
netstat -tlnp | grep youtubebot || echo "Нет активных соединений"
echo ""

# Показываем размер логов
echo "📊 Размер логов:"
sudo journalctl -u youtubebot --disk-usage
echo ""

echo "🔄 Для просмотра логов в реальном времени:"
echo "   sudo journalctl -u youtubebot -f"
