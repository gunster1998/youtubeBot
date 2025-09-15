# 🎬 YouTube Bot - Финальная версия с SOCKS5 прокси

Telegram бот для скачивания видео с YouTube с поддержкой SOCKS5 прокси для обхода блокировки в России.

## ✨ Что нового

- ✅ **SOCKS5 прокси**: Полная поддержка sing-box на сервере
- ✅ **Единая конфигурация**: Все компоненты используют .env
- ✅ **Автоматическое управление**: Прокси включается/выключается через USE_PROXY
- ✅ **Анти-429 защита**: Встроенные задержки для yt-dlp
- ✅ **Самопроверка**: Встроенный тест всех компонентов
- ✅ **Упрощенная настройка**: Один файл .env для всех настроек

## 🚀 Быстрый старт

### 1. Клонирование и настройка

```bash
# Клонируйте репозиторий
git clone https://github.com/gunster1998/youtubeBot.git
cd youtubeBot

# Сделайте скрипты исполняемыми
chmod +x *.sh scripts/*.sh
```

### 2. Настройка прокси

```bash
# Скопируйте конфигурацию
cp env.example .env

# Отредактируйте настройки
nano .env
```

**Настройки в .env:**

```env
# Токен бота
TELEGRAM_BOT_TOKEN=your_bot_token_here

# Настройки прокси
USE_PROXY=true
PROXY_URL=socks5h://127.0.0.1:1080
NO_PROXY=localhost,127.0.0.1,172.16.0.0/12,192.168.0.0/16
```

### 3. Запуск

```bash
# Автоматический запуск с проверками
./quick_start_proxy.sh

# Или ручной запуск
go run cmd/bot/main.go
```

## 🔧 Компоненты с поддержкой прокси

### 1. yt-dlp

Все вызовы yt-dlp автоматически используют прокси:

```go
// Автоматически добавляется --proxy аргумент
args := []string{"--proxy", "socks5h://127.0.0.1:1080"}
```

**Анти-429 задержки:**
- `--sleep-requests 1` - пауза между запросами
- `--sleep-interval 1` - минимальная пауза
- `--max-sleep-interval 3` - максимальная пауза

### 2. HTTP клиенты

Все HTTP клиенты используют единую схему прокси:

```go
// Создание клиента с прокси
client := &http.Client{
    Transport: &http.Transport{
        Proxy: http.ProxyURL(proxyURL),
    },
}
```

### 3. Telegram API

HTTP клиент для Telegram API автоматически использует прокси:

```go
// В main.go
if cfg.Proxy != nil && cfg.Proxy.UseProxy {
    httpClient = cfg.Proxy.CreateHTTPClient()
}
```

### 4. curl команды

Все curl команды используют прокси:

```go
// В CheckNetwork()
args := []string{"--proxy", "socks5h://127.0.0.1:1080"}
```

## 🧪 Тестирование

### Самопроверка

```bash
# Компиляция и запуск
go run scripts/selftest.go

# Или через скрипт
./scripts/run_selftest.sh
```

**Что проверяется:**
- ✅ Настройки прокси из .env
- ✅ HTTP клиент с прокси
- ✅ yt-dlp с прокси
- ✅ curl с прокси
- ✅ Переменные окружения

### Ручное тестирование

```bash
# Тест yt-dlp с прокси
yt-dlp --proxy socks5h://127.0.0.1:1080 -s "https://www.youtube.com/watch?v=dQw4w9WgXcQ"

# Тест curl с прокси
curl --proxy socks5h://127.0.0.1:1080 https://www.youtube.com

# Тест HTTP клиента
go run scripts/selftest.go
```

## 📁 Структура проекта

```
youtubeBot/
├── cmd/bot/              # Основной код бота
├── config/               # Конфигурация
│   ├── config.go         # Основная конфигурация
│   └── proxy.go          # Настройки прокси
├── services/             # Сервисы
│   ├── youtube.go        # YouTube сервис
│   └── universal.go      # Универсальный сервис
├── scripts/              # Скрипты
│   ├── selftest.go       # Самопроверка
│   └── run_selftest.sh   # Запуск самопроверки
├── .env                  # Конфигурация (создается из env.example)
├── env.example           # Пример конфигурации
├── quick_start_proxy.sh  # Быстрый запуск с прокси
└── README_PROXY.md       # Документация по прокси
```

## ⚙️ Конфигурация

### Переменные окружения

| Переменная | Описание | По умолчанию |
|------------|----------|--------------|
| `USE_PROXY` | Включить/выключить прокси | `true` |
| `PROXY_URL` | URL прокси | `socks5h://127.0.0.1:1080` |
| `NO_PROXY` | Исключения для прокси | `localhost,127.0.0.1,172.16.0.0/12,192.168.0.0/16` |
| `TELEGRAM_BOT_TOKEN` | Токен бота | - |
| `TELEGRAM_API_URL` | URL Telegram API | `http://127.0.0.1:8081` |

### Поддерживаемые форматы прокси

- `socks5h://127.0.0.1:1080` - SOCKS5 с DNS через прокси (рекомендуется)
- `socks5://127.0.0.1:1080` - SOCKS5 без DNS через прокси
- `http://127.0.0.1:8080` - HTTP прокси
- `https://127.0.0.1:8080` - HTTPS прокси

## 🔍 Диагностика

### Проблемы с прокси

```bash
# Проверьте статус sing-box
systemctl status sing-box

# Проверьте порт 1080
netstat -tlnp | grep 1080

# Тест подключения
curl --proxy socks5h://127.0.0.1:1080 https://www.google.com
```

### Проблемы с yt-dlp

```bash
# Проверьте yt-dlp
yt-dlp --version

# Тест с прокси
yt-dlp --proxy socks5h://127.0.0.1:1080 -s "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
```

### Проблемы с Telegram API

```bash
# Проверьте локальный сервер
curl http://127.0.0.1:8081/health

# Проверьте статус
systemctl status telegram-bot-api
```

## 🚀 Команды

### Основные команды

```bash
# Быстрый запуск с прокси
./quick_start_proxy.sh

# Самопроверка
./scripts/run_selftest.sh

# Обычный запуск
go run cmd/bot/main.go

# Сборка
go build -o youtubeBot cmd/bot/main.go
```

### Управление сервисами

```bash
# sing-box
sudo systemctl start sing-box
sudo systemctl stop sing-box
sudo systemctl restart sing-box
sudo systemctl status sing-box

# Telegram Bot API
sudo systemctl start telegram-bot-api
sudo systemctl stop telegram-bot-api
sudo systemctl restart telegram-bot-api
sudo systemctl status telegram-bot-api
```

## 📊 Мониторинг

### Логи

```bash
# Логи бота
tail -f bot.log

# Логи sing-box
journalctl -u sing-box -f

# Логи Telegram API
journalctl -u telegram-bot-api -f
```

### Статистика

```bash
# Проверка подключений
ss -tuln | grep 1080

# Мониторинг трафика
iftop -i lo
```

## 🔒 Безопасность

- Прокси работает только локально (127.0.0.1)
- Не открывает порты наружу
- Использует SOCKS5 для обхода блокировок
- DNS запросы идут через прокси (socks5h)
- Автоматические анти-429 задержки

## 🆘 Поддержка

При возникновении проблем:

1. Запустите `./scripts/run_selftest.sh`
2. Проверьте настройки в `.env`
3. Убедитесь что sing-box работает
4. Проверьте логи: `journalctl -u sing-box -f`

## 📄 Лицензия

MIT License - используйте на свой страх и риск!

---

**⚠️ Важно:** Используйте бота только для личных целей и соблюдайте авторские права!


