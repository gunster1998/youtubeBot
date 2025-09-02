# 🚀 Руководство по развертыванию YouTube Bot

## 📋 Требования

### Системные требования:
- **OS:** Linux (Ubuntu 20.04+)
- **RAM:** Минимум 2GB, рекомендуется 4GB+
- **CPU:** 2+ ядра
- **Диск:** 50GB+ свободного места
- **Go:** 1.19+
- **yt-dlp:** Последняя версия

### Зависимости:
```bash
# Установка Go
sudo apt update
sudo apt install golang-go

# Установка yt-dlp
sudo wget -O /usr/local/bin/yt-dlp https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp
sudo chmod +x /usr/local/bin/yt-dlp

# Установка telegram-bot-api
sudo wget -O /usr/local/bin/telegram-bot-api https://github.com/tdlib/telegram-bot-api/releases/download/v7.7.0/telegram-bot-api-linux
sudo chmod +x /usr/local/bin/telegram-bot-api
```

## 🔧 Настройка

### 1. Получение API данных Telegram:
1. Перейдите на https://my.telegram.org
2. Войдите в аккаунт
3. Создайте новое приложение
4. Получите `api_id` и `api_hash`

### 2. Создание бота:
1. Найдите @BotFather в Telegram
2. Создайте нового бота командой `/newbot`
3. Получите токен бота

### 3. Настройка конфигурации:
```bash
# Создайте файл config.env
cat > config.env << EOF
TELEGRAM_BOT_TOKEN=your_bot_token_here
TELEGRAM_API_URL=http://127.0.0.1:8081
API_ID=your_api_id_here
API_HASH=your_api_hash_here
EOF
```

## 🚀 Развертывание

### 1. Клонирование репозитория:
```bash
git clone https://github.com/gunster1998/youtubeBot.git
cd youtubeBot
```

### 2. Установка зависимостей:
```bash
go mod tidy
go mod download
```

### 3. Запуск локального сервера Telegram API:
```bash
# Запуск в фоне
nohup /usr/local/bin/telegram-bot-api \
  --api-id=YOUR_API_ID \
  --api-hash=YOUR_API_HASH \
  --local \
  --http-port=8081 \
  --http-ip-address=127.0.0.1 \
  > telegram-api.log 2>&1 &

# Проверка запуска
sleep 3
ps aux | grep telegram-bot-api
netstat -tulpn | grep :8081
```

### 4. Запуск бота:
```bash
# Быстрый запуск
./quick_start.sh

# Или вручную
./run_async.sh
```

## 🔍 Проверка работы

### 1. Проверка процессов:
```bash
# Проверка telegram-bot-api
ps aux | grep telegram-bot-api

# Проверка бота
ps aux | grep youtubeBot_async

# Проверка портов
netstat -tulpn | grep :8081
```

### 2. Проверка логов:
```bash
# Логи telegram-bot-api
tail -f telegram-api.log

# Логи бота (в консоли)
# Бот выводит логи в stdout
```

### 3. Тестирование бота:
1. Найдите бота в Telegram по имени
2. Отправьте команду `/start`
3. Отправьте ссылку на YouTube видео
4. Выберите формат
5. Дождитесь загрузки

## 🛠️ Управление

### Остановка сервисов:
```bash
# Остановка бота
pkill -f youtubeBot_async

# Остановка telegram-bot-api
pkill -f telegram-bot-api
```

### Перезапуск:
```bash
# Перезапуск бота
pkill -f youtubeBot_async
./run_async.sh

# Перезапуск telegram-bot-api
pkill -f telegram-bot-api
nohup /usr/local/bin/telegram-bot-api \
  --api-id=YOUR_API_ID \
  --api-hash=YOUR_API_HASH \
  --local \
  --http-port=8081 \
  --http-ip-address=127.0.0.1 \
  > telegram-api.log 2>&1 &
```

### Обновление:
```bash
# Остановка сервисов
pkill -f youtubeBot_async
pkill -f telegram-bot-api

# Обновление кода
git pull origin main

# Пересборка и запуск
./quick_start.sh
```

## 📊 Мониторинг

### Статистика системы:
```bash
# Использование CPU и памяти
htop

# Использование диска
df -h

# Сетевые соединения
netstat -tulpn | grep :8081
```

### Логи бота:
- **Успешные загрузки:** `✅ Видео успешно отправлено!`
- **Ошибки:** `❌ Ошибка отправки видео`
- **Кэш:** `⚡ Видео найдено в кэше`
- **Очередь:** `📝 Задача добавлена в очередь`

## 🔧 Устранение неполадок

### Проблема: "connection refused" на порту 8081
```bash
# Решение: Запустить telegram-bot-api
nohup /usr/local/bin/telegram-bot-api \
  --api-id=YOUR_API_ID \
  --api-hash=YOUR_API_HASH \
  --local \
  --http-port=8081 \
  --http-ip-address=127.0.0.1 \
  > telegram-api.log 2>&1 &
```

### Проблема: "yt-dlp not found"
```bash
# Решение: Установить yt-dlp
sudo wget -O /usr/local/bin/yt-dlp https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp
sudo chmod +x /usr/local/bin/yt-dlp
```

### Проблема: "missing go.sum entry"
```bash
# Решение: Обновить зависимости
go get github.com/mattn/go-sqlite3
go mod tidy
```

### Проблема: Видео не отправляются
```bash
# Проверка: Логи бота
# Ищите сообщения:
# - "✅ Задача завершена успешно"
# - "❌ Ошибка отправки видео"
# - "⚠️ Задача не найдена в активных"
```

## 📈 Оптимизация

### Настройки производительности:
```go
// В services/queue.go
downloadQueue := services.NewDownloadQueue(5, youtubeService, cacheService) // Увеличить воркеров

// В cmd/bot/main_async.go
timeout := time.NewTimer(15 * time.Minute) // Увеличить таймаут
```

### Рекомендации по ресурсам:
- **CPU:** 4+ ядер для 5+ воркеров
- **RAM:** 4GB+ для больших видео
- **Диск:** SSD для быстрого кэша
- **Сеть:** Стабильное соединение

## 🔒 Безопасность

### Рекомендации:
1. **Firewall:** Открыть только необходимые порты
2. **Права доступа:** Ограничить права пользователя
3. **Логи:** Регулярно очищать старые логи
4. **Обновления:** Следить за обновлениями зависимостей

### Настройка firewall:
```bash
# Разрешить только локальные соединения
sudo ufw allow from 127.0.0.1 to any port 8081
sudo ufw deny 8081
```

---

**🎉 Готово!** Бот развернут и готов к работе с множественными пользователями!
