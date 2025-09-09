#!/bin/bash

echo "📝 Логи YouTube Bot сервиса"
echo "=========================="

# Показываем последние логи
echo "🔍 Последние 50 строк логов:"
sudo journalctl -u youtubebot -n 50 --no-pager

echo ""
echo "📊 Статистика логов:"
echo "  Всего записей: $(sudo journalctl -u youtubebot --no-pager | wc -l)"
echo "  Размер логов: $(sudo journalctl -u youtubebot --disk-usage)"
echo "  Последний запуск: $(sudo journalctl -u youtubebot --since today | head -1)"

echo ""
echo "🔧 Полезные команды:"
echo "  sudo journalctl -u youtubebot -f              # Логи в реальном времени"
echo "  sudo journalctl -u youtubebot --since today   # Логи за сегодня"
echo "  sudo journalctl -u youtubebot --since '1 hour ago'  # Логи за последний час"
echo "  sudo journalctl -u youtubebot --since '2024-01-01'  # Логи с даты"
echo "  sudo journalctl -u youtubebot -n 100          # Последние 100 строк"
