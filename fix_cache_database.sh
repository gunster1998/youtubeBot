#!/bin/bash

echo "🔧 Исправление базы данных кэша"
echo "================================"

# Переходим в папку с ботом
cd /home/sergey/GOLANG/youtubebotv3

# Проверяем, существует ли база данных
if [ -f "downloads/video_cache.db" ]; then
    echo "📁 Найдена база данных: downloads/video_cache.db"
    
    # Создаем резервную копию
    echo "💾 Создаю резервную копию..."
    cp downloads/video_cache.db downloads/video_cache.db.backup
    
    # Очищаем базу данных
    echo "🗑️ Очищаю старую базу данных..."
    rm downloads/video_cache.db
    
    echo "✅ База данных очищена. При следующем запуске бота будет создана новая с правильной структурой."
else
    echo "ℹ️ База данных не найдена. При следующем запуске бота будет создана новая."
fi

echo ""
echo "🚀 Теперь можно запускать бота:"
echo "   ./run.sh"
echo ""
echo "📝 Изменения:"
echo "   - Исправлена структура базы данных"
echo "   - Добавлен UNIQUE constraint для (video_id, platform, format_id)"
echo "   - Исправлена логика добавления в кэш"
echo "   - Теперь каждый формат будет кэшироваться отдельно"
