#!/bin/bash

echo "🚀 Установка YouTube Bot как сервис..."

# Останавливаем сервис если запущен
sudo systemctl stop youtubebot 2>/dev/null || true

# Копируем сервис файл
sudo cp youtubebot.service /etc/systemd/system/

# Перезагружаем systemd
sudo systemctl daemon-reload

# Включаем автозапуск
sudo systemctl enable youtubebot

# Запускаем сервис
sudo systemctl start youtubebot

# Проверяем статус
echo "📊 Статус сервиса:"
sudo systemctl status youtubebot --no-pager

echo ""
echo "✅ Сервис установлен и запущен!"
echo ""
echo "🔧 Управление сервисом:"
echo "  sudo systemctl start youtubebot    # Запустить"
echo "  sudo systemctl stop youtubebot     # Остановить"
echo "  sudo systemctl restart youtubebot  # Перезапустить"
echo "  sudo systemctl status youtubebot   # Статус"
echo "  sudo journalctl -u youtubebot -f   # Логи в реальном времени"
echo ""
echo "📝 Логи сервиса:"
echo "  sudo journalctl -u youtubebot --since today"
