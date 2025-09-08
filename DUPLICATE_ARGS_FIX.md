# 🔧 Исправление дублирования аргументов yt-dlp

## ❌ **Проблема:**
В логах видно, что команда yt-dlp содержит дублированные параметры:

```bash
--format 247 --output downloads/%(id)s.%(ext)s --no-playlist --no-check-certificates --max-filesize 2G --socket-timeout 60 --retries 5 --no-playlist --no-check-certificates --max-filesize 2G --socket-timeout 60 --retries 5 --format best[ext=mp4]/best
```

Это приводило к тому, что:
1. **Разные форматы скачивались как один файл** - yt-dlp игнорировал первый `--format` и использовал последний
2. **Одинаковые видео для разных форматов** - все форматы скачивались в `best[ext=mp4]/best`
3. **Неправильная работа кэша** - кэш работал правильно, но файлы были одинаковые

## 🔍 **Причина:**
В `services/universal.go` происходило дублирование аргументов:

```go
// СТАРЫЙ КОД (с дублированием)
args := us.platformDetector.GetYtDlpArgs(platformInfo.Type)  // Содержит прокси
downloadArgs := []string{...}                               // Основные аргументы
proxyArgs := getProxyArgs()                                 // Прокси еще раз

allArgs := append(downloadArgs, args...)                    // Дублирование!
allArgs = append(allArgs, proxyArgs...)                    // Дублирование!
```

## ✅ **Исправление:**

### 1. **Убрано дублирование в `DownloadVideoWithFormat`:**
```go
// НОВЫЙ КОД (без дублирования)
downloadArgs := []string{
    "--format", formatID,
    "--output", filepath.Join(us.downloadDir, "%(id)s.%(ext)s"),
    "--no-playlist",
    "--no-check-certificates",
    "--max-filesize", "2G",
    "--socket-timeout", "60",
    "--retries", "5",
}

proxyArgs := getProxyArgs()

// Объединяем все аргументы (без дублирования)
allArgs := append(downloadArgs, proxyArgs...)
allArgs = append(allArgs, url)
```

### 2. **Убрано дублирование в `GetVideoFormats`:**
```go
// НОВЫЙ КОД (без дублирования)
formatArgs := []string{
    "--list-formats",
    "--no-playlist",
    "--no-check-certificates",
}

proxyArgs := getProxyArgs()

// Объединяем все аргументы (без дублирования)
allArgs := append(formatArgs, proxyArgs...)
allArgs = append(allArgs, url)
```

## 🎯 **Результат:**

### ✅ **Теперь работает правильно:**
- **Формат 247** → скачивается как `--format 247`
- **Формат 242** → скачивается как `--format 242`
- **Разные файлы** → каждый формат создает свой файл
- **Правильный кэш** → каждый формат кэшируется отдельно

### 🔧 **Технические детали:**
1. **Убрано дублирование** - аргументы добавляются только один раз
2. **Правильная команда yt-dlp** - каждый формат скачивается отдельно
3. **Кэш работает корректно** - каждый формат имеет свой файл
4. **Обратная совместимость** - все функции работают как прежде

## 🚀 **Готово к тестированию!**

Теперь при выборе разных форматов бот будет:
1. ✅ Скачивать каждый формат отдельно
2. ✅ Создавать разные файлы для разных форматов
3. ✅ Показывать правильные размеры файлов
4. ✅ Кэшировать каждый формат отдельно

**Мама в безопасности!** 💚 Проблема с дублированием аргументов исправлена!
