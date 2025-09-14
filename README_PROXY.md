# 🌐 Настройка прокси для YouTube Bot

Этот документ описывает настройку SOCKS5 прокси для обхода блокировки YouTube в России.

## 📋 Обзор

Бот поддерживает работу через SOCKS5 прокси (sing-box на сервере) для обхода блокировки YouTube. Все сетевые запросы (Telegram API, yt-dlp, HTTP клиенты) могут быть направлены через прокси.

## ⚙️ Конфигурация

### Переменные окружения

Создайте файл `.env` на основе `env.example`:

```bash
cp env.example .env
nano .env
```

**Основные настройки прокси:**

```env
# Включить/выключить прокси
USE_PROXY=true

# URL прокси (sing-box на сервере)
PROXY_URL=socks5h://127.0.0.1:1080

# Исключения для прокси
NO_PROXY=localhost,127.0.0.1,172.16.0.0/12,192.168.0.0/16
```

### Поддерживаемые форматы прокси

- `socks5h://127.0.0.1:1080` - SOCKS5 с DNS через прокси (рекомендуется)
- `socks5://127.0.0.1:1080` - SOCKS5 без DNS через прокси
- `http://127.0.0.1:8080` - HTTP прокси
- `https://127.0.0.1:8080` - HTTPS прокси

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

Запустите встроенную самопроверку:

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

## 🚀 Быстрый старт

### 1. Настройка прокси

```bash
# Скопируйте конфигурацию
cp env.example .env

# Отредактируйте настройки
nano .env

# Установите USE_PROXY=true и PROXY_URL=socks5h://127.0.0.1:1080
```

### 2. Запуск бота

```bash
# Обычный запуск
go run cmd/bot/main.go

# Или через скрипт
./run.sh
```

### 3. Проверка работы

```bash
# Самопроверка
./scripts/run_selftest.sh

# Проверка логов
tail -f bot.log
```

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

# Проверьте логи
yt-dlp --verbose --proxy socks5h://127.0.0.1:1080 "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
```

### Проблемы с Telegram API

```bash
# Проверьте локальный сервер
curl http://127.0.0.1:8081/health

# Проверьте статус
systemctl status telegram-bot-api
```

## ⚙️ Дополнительные настройки

### Отключение прокси

```env
# В .env файле
USE_PROXY=false
```

### Изменение прокси

```env
# В .env файле
PROXY_URL=socks5h://127.0.0.1:1080
```

### Исключения для прокси

```env
# Не проксировать эти адреса
NO_PROXY=localhost,127.0.0.1,172.16.0.0/12,192.168.0.0/16
```

## 📊 Мониторинг

### Логи прокси

```bash
# Логи sing-box
journalctl -u sing-box -f

# Логи бота
tail -f bot.log | grep -i proxy
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

## 🆘 Поддержка

При возникновении проблем:

1. Запустите `./scripts/run_selftest.sh`
2. Проверьте настройки в `.env`
3. Убедитесь что sing-box работает
4. Проверьте логи: `journalctl -u sing-box -f`

---

**⚠️ Важно:** Используйте прокси только для обхода блокировок в вашей стране!

