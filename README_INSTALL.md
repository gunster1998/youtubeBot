# 📦 Установка YouTube Bot с SOCKS5 прокси

Пошаговые инструкции по установке и настройке YouTube Bot с поддержкой SOCKS5 прокси.

## 🔧 Установка зависимостей

### 1. Go

```bash
# Ubuntu/Debian
sudo apt update
sudo apt install golang-go

# Или скачайте с https://golang.org/dl/
wget https://go.dev/dl/go1.22.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.22.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

### 2. yt-dlp

```bash
# Через pip
pip install yt-dlp

# Или через apt
sudo apt install yt-dlp

# Или скачайте напрямую
sudo wget -O /usr/local/bin/yt-dlp https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp
sudo chmod +x /usr/local/bin/yt-dlp
```

### 3. curl

```bash
# Ubuntu/Debian
sudo apt install curl

# CentOS/RHEL
sudo yum install curl
```

### 4. ffmpeg (опционально)

```bash
# Ubuntu/Debian
sudo apt install ffmpeg

# CentOS/RHEL
sudo yum install ffmpeg
```

## 🚀 Установка бота

### 1. Клонирование

```bash
git clone https://github.com/gunster1998/youtubeBot.git
cd youtubeBot
chmod +x *.sh scripts/*.sh
```

### 2. Настройка конфигурации

```bash
# Скопируйте конфигурацию
cp env.example .env

# Отредактируйте настройки
nano .env
```

**Настройки в .env:**

```env
# Токен бота (получите у @BotFather)
TELEGRAM_BOT_TOKEN=your_bot_token_here

# Настройки прокси
USE_PROXY=true
PROXY_URL=socks5h://127.0.0.1:1080
NO_PROXY=localhost,127.0.0.1,172.16.0.0/12,192.168.0.0/16

# Telegram API
TELEGRAM_API_URL=http://127.0.0.1:8081
HTTP_TIMEOUT=60
DOWNLOAD_DIR=./downloads
MAX_FILE_SIZE=0
```

### 3. Настройка sing-box (SOCKS5 прокси)

```bash
# Установите sing-box
sudo bash -c "$(curl -L https://sing-box.sagernet.org/install.sh)"

# Создайте конфигурацию
sudo mkdir -p /etc/sing-box
sudo tee /etc/sing-box/config.json > /dev/null << 'EOF'
{
  "log": {
    "level": "info"
  },
  "inbounds": [
    {
      "type": "mixed",
      "listen": "127.0.0.1",
      "listen_port": 1080
    }
  ],
  "outbounds": [
    {
      "type": "direct"
    }
  ]
}
EOF

# Запустите sing-box
sudo systemctl enable sing-box
sudo systemctl start sing-box
```

### 4. Настройка Telegram Bot API

```bash
# Установите Telegram Bot API
wget https://github.com/tdlib/telegram-bot-api/releases/download/v7.0.0/telegram-bot-api_7.0.0_linux_amd64.tar.gz
tar -xzf telegram-bot-api_7.0.0_linux_amd64.tar.gz
sudo mv telegram-bot-api /usr/local/bin/
sudo chmod +x /usr/local/bin/telegram-bot-api

# Создайте systemd сервис
sudo tee /etc/systemd/system/telegram-bot-api.service > /dev/null << 'EOF'
[Unit]
Description=Telegram Bot API
After=network.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/telegram-bot-api --local --http-ip-address=127.0.0.1 --http-port=8081 --dir=/var/lib/telegram-bot-api --temp-dir=/var/tmp/telegram-bot-api
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# Запустите сервис
sudo systemctl daemon-reload
sudo systemctl enable telegram-bot-api
sudo systemctl start telegram-bot-api
```

## 🧪 Тестирование

### 1. Самопроверка

```bash
# Запустите самопроверку
./scripts/run_selftest.sh
```

### 2. Ручное тестирование

```bash
# Тест прокси
curl --proxy socks5h://127.0.0.1:1080 https://www.google.com

# Тест yt-dlp
yt-dlp --proxy socks5h://127.0.0.1:1080 -s "https://www.youtube.com/watch?v=dQw4w9WgXcQ"

# Тест Telegram API
curl http://127.0.0.1:8081/health
```

## 🚀 Запуск

### Быстрый запуск

```bash
# Автоматический запуск с проверками
./quick_start_proxy.sh
```

### Ручной запуск

```bash
# Сборка
go build -o youtubeBot cmd/bot/main.go

# Запуск
./youtubeBot
```

## 🔍 Диагностика

### Проверка сервисов

```bash
# Статус всех сервисов
sudo systemctl status sing-box telegram-bot-api

# Логи
sudo journalctl -u sing-box -f
sudo journalctl -u telegram-bot-api -f
```

### Проверка портов

```bash
# Проверка портов
netstat -tlnp | grep -E "(1080|8081)"

# Тест подключения
telnet 127.0.0.1 1080
telnet 127.0.0.1 8081
```

### Проверка прокси

```bash
# Тест SOCKS5
curl --proxy socks5h://127.0.0.1:1080 https://www.google.com

# Тест HTTP
curl --proxy http://127.0.0.1:1080 https://www.google.com
```

## 🛠️ Устранение неполадок

### Проблема: yt-dlp не найден

```bash
# Установите yt-dlp
pip install yt-dlp

# Или скачайте напрямую
sudo wget -O /usr/local/bin/yt-dlp https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp
sudo chmod +x /usr/local/bin/yt-dlp
```

### Проблема: Прокси не работает

```bash
# Проверьте sing-box
sudo systemctl status sing-box

# Перезапустите
sudo systemctl restart sing-box

# Проверьте конфигурацию
sudo cat /etc/sing-box/config.json
```

### Проблема: Telegram API не работает

```bash
# Проверьте сервис
sudo systemctl status telegram-bot-api

# Перезапустите
sudo systemctl restart telegram-bot-api

# Проверьте порт
netstat -tlnp | grep 8081
```

### Проблема: Ошибка 409 в боте

```bash
# Удалите старый offset
rm -f last_offset.txt

# Перезапустите бота
./youtubeBot
```

## 📊 Мониторинг

### Логи

```bash
# Логи бота
tail -f bot.log

# Логи sing-box
sudo journalctl -u sing-box -f

# Логи Telegram API
sudo journalctl -u telegram-bot-api -f
```

### Статистика

```bash
# Подключения
ss -tuln | grep -E "(1080|8081)"

# Трафик
iftop -i lo

# Процессы
ps aux | grep -E "(youtubeBot|sing-box|telegram-bot-api)"
```

## 🔒 Безопасность

- Прокси работает только локально (127.0.0.1)
- Не открывает порты наружу
- Использует SOCKS5 для обхода блокировок
- DNS запросы идут через прокси (socks5h)

## 🆘 Поддержка

При возникновении проблем:

1. Запустите `./scripts/run_selftest.sh`
2. Проверьте настройки в `.env`
3. Убедитесь что все сервисы запущены
4. Проверьте логи: `sudo journalctl -u sing-box -f`

---

**⚠️ Важно:** Используйте бота только для личных целей и соблюдайте авторские права!


