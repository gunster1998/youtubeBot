#!/bin/bash

# Скрипт для копирования исправленного проекта на сервер

echo "🚀 Подготовка проекта для деплоя на сервер..."

# Создаем архив с исправленным проектом
echo "📦 Создание архива..."
tar -czf youtubeBot_fixed.tar.gz \
    --exclude='.git' \
    --exclude='downloads' \
    --exclude='*.log' \
    --exclude='youtubeBot' \
    --exclude='youtubeBot_async' \
    --exclude='test_build' \
    .

echo "✅ Архив создан: youtubeBot_fixed.tar.gz"

echo ""
echo "📋 Инструкции для сервера:"
echo "=========================="
echo ""
echo "1. Скопируйте архив на сервер:"
echo "   scp youtubeBot_fixed.tar.gz gunster1998@your-server:~/repos/"
echo ""
echo "2. На сервере распакуйте архив:"
echo "   cd ~/repos"
echo "   tar -xzf youtubeBot_fixed.tar.gz -C youtubeBot/"
echo ""
echo "3. Перейдите в папку проекта:"
echo "   cd ~/repos/youtubeBot"
echo ""
echo "4. Дайте права на выполнение:"
echo "   chmod +x *.sh"
echo ""
echo "5. Запустите бота:"
echo "   ./run_simple.sh"
echo ""
echo "🎉 Готово!"


