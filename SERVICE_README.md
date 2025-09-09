# 🚀 YouTube Bot - Запуск как сервис

## 📋 Быстрый старт

### 1. Установка сервиса
```bash
chmod +x *.sh
./start_service.sh
```

### 2. Проверка статуса
```bash
./monitor_service.sh
```

### 3. Просмотр логов
```bash
./logs_service.sh
```

## 🔧 Управление сервисом

### Основные команды
```bash
# Запустить сервис
sudo systemctl start youtubebot

# Остановить сервис
sudo systemctl stop youtubebot

# Перезапустить сервис
sudo systemctl restart youtubebot

# Статус сервиса
sudo systemctl status youtubebot

# Включить автозапуск
sudo systemctl enable youtubebot

# Отключить автозапуск
sudo systemctl disable youtubebot
```

### Просмотр логов
```bash
# Логи в реальном времени
sudo journalctl -u youtubebot -f

# Последние 100 строк
sudo journalctl -u youtubebot -n 100

# Логи за сегодня
sudo journalctl -u youtubebot --since today

# Логи за последний час
sudo journalctl -u youtubebot --since '1 hour ago'

# Логи с определенной даты
sudo journalctl -u youtubebot --since '2024-01-01'
```

## 🔄 Обновление

### Автоматическое обновление
```bash
./update_service.sh
```

### Ручное обновление
```bash
# Остановить сервис
sudo systemctl stop youtubebot

# Обновить код
cd /home/gunster1998/repos/youtubeBot
git pull origin main

# Собрать проект
go mod tidy
go build -o youtubeBot cmd/bot/main.go

# Запустить сервис
sudo systemctl start youtubebot
```

## 📊 Мониторинг

### Проверка здоровья
```bash
# Статус сервиса
sudo systemctl is-active youtubebot

# Использование ресурсов
ps aux | grep youtubebot

# Открытые порты
netstat -tlnp | grep youtubebot

# Размер логов
sudo journalctl -u youtubebot --disk-usage
```

### Автоматический мониторинг
```bash
# Добавить в crontab для проверки каждые 5 минут
*/5 * * * * /home/gunster1998/repos/youtubeBot/monitor_service.sh >> /var/log/youtubebot_monitor.log 2>&1
```

## 🛠️ Настройка

### Конфигурация
- Файл конфигурации: `config.env`
- Рабочая директория: `/home/gunster1998/repos/youtubeBot`
- Пользователь: `gunster1998`
- Группа: `gunster1998`

### Ограничения ресурсов
- Максимум файлов: 65536
- Максимум процессов: 4096
- GOMAXPROCS: 4
- GOGC: 100

## 🚨 Устранение неполадок

### Сервис не запускается
```bash
# Проверить логи
sudo journalctl -u youtubebot -n 50

# Проверить права доступа
ls -la /home/gunster1998/repos/youtubeBot/youtubeBot

# Проверить конфигурацию
cat /home/gunster1998/repos/youtubeBot/config.env
```

### Сервис падает
```bash
# Проверить логи ошибок
sudo journalctl -u youtubebot --since '1 hour ago' | grep -i error

# Проверить использование памяти
free -h

# Проверить диск
df -h
```

### Высокое использование ресурсов
```bash
# Проверить процессы
top -p $(pgrep youtubebot)

# Проверить открытые файлы
lsof -p $(pgrep youtubebot)

# Перезапустить сервис
sudo systemctl restart youtubebot
```

## 📈 Производительность

### Оптимизация
- Увеличить лимиты в `youtubebot.service`
- Настроить `GOMAXPROCS` под количество ядер
- Настроить `GOGC` для управления сборкой мусора
- Использовать SSD для директории загрузок

### Мониторинг производительности
```bash
# CPU и память
htop

# Диск
iotop

# Сеть
iftop

# Логи производительности
sudo journalctl -u youtubebot | grep "МЕТРИКИ"
```

## 🔒 Безопасность

### Рекомендации
- Регулярно обновлять Go и зависимости
- Мониторить логи на подозрительную активность
- Использовать firewall для ограничения доступа
- Регулярно очищать старые логи
- Делать резервные копии конфигурации

### Очистка логов
```bash
# Очистить логи старше 7 дней
sudo journalctl --vacuum-time=7d

# Ограничить размер логов до 100MB
sudo journalctl --vacuum-size=100M
```
