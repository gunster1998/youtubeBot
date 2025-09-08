#!/bin/bash

echo "🔨 Тестирование сборки проекта..."

# Очистка кэша модулей
go clean -modcache

# Инициализация модулей
go mod init youtubeBot 2>/dev/null || true

# Загрузка зависимостей
go mod tidy

# Тестирование импортов
echo "📦 Тестирование импортов..."
go build -o test_build test_build.go

if [ $? -eq 0 ]; then
    echo "✅ Импорты работают!"
    rm -f test_build
else
    echo "❌ Ошибка импортов"
    exit 1
fi

# Тестирование сборки основного файла
echo "🔨 Тестирование сборки main.go..."
go build -o youtubeBot cmd/bot/main.go

if [ $? -eq 0 ]; then
    echo "✅ main.go собирается успешно!"
    rm -f youtubeBot
else
    echo "❌ Ошибка сборки main.go"
    exit 1
fi

echo "🎉 Все тесты прошли успешно!"


