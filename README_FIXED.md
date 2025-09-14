# 🎬 YouTube Bot - Исправленная версия

Telegram бот для скачивания видео с YouTube с поддержкой VLESS-Reality прокси для обхода блокировки в России.

## ✨ Что исправлено

- ✅ **Версия Go**: Исправлена с 1.25.0 на 1.22 для совместимости
- ✅ **Ошибка 409**: Добавлена обработка конфликтов в getUpdates
- ✅ **Offset управление**: Автоматическое сохранение и загрузка offset
- ✅ **VLESS прокси**: Полная настройка для обхода блокировки YouTube
- ✅ **Автоматизация**: Скрипты для установки и настройки

## 🚀 Быстрый старт

### 1. Клонирование и настройка

```bash
# Клонируйте репозиторий
git clone https://github.com/gunster1998/youtubeBot.git
cd youtubeBot

# Сделайте скрипты исполняемыми
chmod +x *.sh
```

### 2. Быстрый запуск (рекомендуется)

```bash
# Автоматическая установка и запуск
./quick_start.sh
```

### 3. Ручная настройка

```bash
# 1. Обновите конфигурацию
./update_config_for_vless.sh

# 2. Установите VLESS прокси
./install_vless_proxy.sh

# 3. Протестируйте подключение
./test_proxy_connection.sh

# 4. Запустите бота
./youtubeBot
```

## 🔧 Исправления

### Ошибка 409 Conflict

**Проблема**: `неуспешный статус getUpdates: 409`

**Решение**:
- Добавлено автоматическое сохранение offset в `last_offset.txt`
- При ошибке 409 offset сбрасывается на 0
- Timeout изменен с 30 на 0 для стабильности

### Версия Go

**Проблема**: `go.mod requires go >= 1.25.0 (running go 1.22.2)`

**Решение**:
- Изменена версия в `go.mod` с 1.25.0 на 1.22
- Совместимость с Go 1.22+

### VLESS прокси

**Проблема**: YouTube заблокирован в России

**Решение**:
- Автоматическая установка Xray
- Настройка VLESS-Reality клиента
- Локальные прокси порты: SOCKS5 (10808), HTTP (10809)

## 📁 Новые файлы

- `install_vless_proxy.sh` - Установка VLESS прокси
- `test_proxy_connection.sh` - Тестирование подключения
- `update_config_for_vless.sh` - Обновление конфигурации
- `quick_start.sh` - Быстрый запуск
- `README_VLESS_SETUP.md` - Подробная документация по VLESS

## 🌐 VLESS конфигурация

Используется VLESS-Reality прокси:

```
vless://acd38d98-9bd7-41f6-baca-592a2c630ec9@paris1.chillvpn2.ru:443?type=tcp&security=reality&pbk=J3WdjsVkO93dMknXmEQk8_WE2opdL4AEMu77BLsE0Dk&fp=chrome&sni=yahoo.com&sid=cf&spx=%2F&flow=xtls-rprx-vision-udp443
```

**Параметры**:
- UUID: `acd38d98-9bd7-41f6-baca-592a2c630ec9`
- Server: `paris1.chillvpn2.ru:443`
- Public Key: `J3WdjsVkO93dMknXmEQk8_WE2opdL4AEMu77BLsE0Dk`
- SNI: `yahoo.com`
- Short ID: `cf`

## 🔧 Управление

### Основные команды

```bash
# Быстрый запуск
./quick_start.sh

# Установка прокси
./install_vless_proxy.sh

# Тестирование
./test_proxy_connection.sh

# Обновление конфига
./update_config_for_vless.sh

# Запуск бота
./youtubeBot
```

### Управление сервисами

```bash
# Xray сервис
sudo systemctl start xray
sudo systemctl stop xray
sudo systemctl restart xray
sudo systemctl status xray

# Telegram Bot API
sudo systemctl start telegram-bot-api
sudo systemctl stop telegram-bot-api
sudo systemctl restart telegram-bot-api
sudo systemctl status telegram-bot-api
```

## 🧪 Тестирование

### Проверка прокси

```bash
# Полный тест
./test_proxy_connection.sh

# Ручная проверка
curl --proxy socks5://127.0.0.1:10808 https://www.youtube.com
curl --proxy http://127.0.0.1:10809 https://www.youtube.com
```

### Проверка бота

```bash
# Запуск с логами
./youtubeBot 2>&1 | tee bot.log

# Проверка offset
cat last_offset.txt
```

## 🔍 Диагностика

### Проблемы с прокси

```bash
# Статус Xray
sudo systemctl status xray

# Логи Xray
sudo journalctl -u xray -f

# Проверка портов
netstat -tlnp | grep -E "(10808|10809)"
```

### Проблемы с ботом

```bash
# Ошибка 409
rm -f last_offset.txt
./youtubeBot

# Проблемы с Telegram API
curl http://127.0.0.1:8081/health
sudo systemctl status telegram-bot-api
```

## 📊 Мониторинг

### Логи

```bash
# Логи бота
tail -f bot.log

# Логи Xray
sudo journalctl -u xray -f

# Логи Telegram API
sudo journalctl -u telegram-bot-api -f
```

### Статус

```bash
# Проверка всех сервисов
./test_proxy_connection.sh

# Только статус
sudo systemctl status xray telegram-bot-api
```

## 🎯 Результат

После исправлений:

- ✅ Бот работает стабильно без ошибок 409
- ✅ YouTube доступен через VLESS прокси
- ✅ Автоматическая установка и настройка
- ✅ Подробная диагностика и мониторинг
- ✅ Совместимость с Go 1.22+

## 🆘 Поддержка

При возникновении проблем:

1. Запустите `./test_proxy_connection.sh`
2. Проверьте логи: `sudo journalctl -u xray -f`
3. Убедитесь что все сервисы запущены
4. Проверьте интернет соединение

## 📄 Лицензия

MIT License - используйте на свой страх и риск!

---

**⚠️ Важно:** Используйте бота только для личных целей и соблюдайте авторские права!
