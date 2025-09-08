package services

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
	_ "github.com/mattn/go-sqlite3"
)

// VideoCache представляет кэшированное видео
type VideoCache struct {
	ID           int64
	VideoID      string    // Video ID (любой платформы)
	Platform     string    // Тип платформы (youtube, tiktok, instagram, etc.)
	URL          string    // Полный URL
	Title        string    // Название видео
	DownloadCount int       // Количество скачиваний
	LastDownload time.Time // Последнее скачивание
	FileSize     int64     // Размер файла в байтах
	FilePath     string    // Путь к файлу на диске
	FormatID     string    // ID формата
	Resolution   string    // Разрешение
	CreatedAt    time.Time // Дата создания
}

// CacheService управляет кэшированием видео
type CacheService struct {
	db          *sql.DB
	cacheDir    string
	maxCacheSize int64 // Максимальный размер кэша в байтах (20-30 ГБ)
	mutex       sync.RWMutex // Защита от race conditions
}

// NewCacheService создает новый сервис кэширования
func NewCacheService(cacheDir string, maxCacheSizeGB int) (*CacheService, error) {
	// Создаем директорию кэша если не существует
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("ошибка создания директории кэша: %v", err)
	}

	// Инициализируем базу данных
	dbPath := filepath.Join(cacheDir, "video_cache.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия БД: %v", err)
	}

	// Создаем таблицу если не существует
	if err := createCacheTable(db); err != nil {
		return nil, fmt.Errorf("ошибка создания таблицы: %v", err)
	}

	service := &CacheService{
		db:          db,
		cacheDir:    cacheDir,
		maxCacheSize: int64(maxCacheSizeGB) * 1024 * 1024 * 1024, // Конвертируем в байты
	}

	// Очищаем старые файлы при запуске
	if err := service.cleanupOldFiles(); err != nil {
		log.Printf("⚠️ Предупреждение: не удалось очистить старые файлы: %v", err)
	}

	return service, nil
}

// createCacheTable создает таблицу для кэша
func createCacheTable(db *sql.DB) error {
	// Сначала создаем таблицу если не существует
	createQuery := `
	CREATE TABLE IF NOT EXISTS video_cache (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		video_id TEXT NOT NULL,
		platform TEXT NOT NULL DEFAULT 'youtube',
		url TEXT NOT NULL,
		title TEXT NOT NULL,
		download_count INTEGER DEFAULT 1,
		last_download DATETIME DEFAULT CURRENT_TIMESTAMP,
		file_size INTEGER NOT NULL,
		file_path TEXT NOT NULL,
		format_id TEXT NOT NULL,
		resolution TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(video_id, platform, format_id)
	);
	`
	
	_, err := db.Exec(createQuery)
	if err != nil {
		return err
	}
	
	// Проверяем, существует ли колонка platform
	var count int
	checkQuery := `SELECT COUNT(*) FROM pragma_table_info('video_cache') WHERE name='platform'`
	err = db.QueryRow(checkQuery).Scan(&count)
	if err != nil {
		return fmt.Errorf("ошибка проверки колонки platform: %v", err)
	}
	
	// Если колонка platform не существует, добавляем её
	if count == 0 {
		log.Printf("🔄 Добавляю колонку platform в существующую таблицу...")
		
		// Добавляем колонку platform
		alterQuery := `ALTER TABLE video_cache ADD COLUMN platform TEXT NOT NULL DEFAULT 'youtube'`
		_, err = db.Exec(alterQuery)
		if err != nil {
			return fmt.Errorf("ошибка добавления колонки platform: %v", err)
		}
		
		// Обновляем существующие записи
		updateQuery := `UPDATE video_cache SET platform = 'youtube' WHERE platform IS NULL OR platform = ''`
		_, err = db.Exec(updateQuery)
		if err != nil {
			return fmt.Errorf("ошибка обновления существующих записей: %v", err)
		}
		
		log.Printf("✅ Колонка platform добавлена успешно")
	}
	
	// Проверяем, существует ли UNIQUE constraint
	var constraintCount int
	constraintQuery := `SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_video_platform_format'`
	err = db.QueryRow(constraintQuery).Scan(&constraintCount)
	if err != nil {
		log.Printf("⚠️ Предупреждение: не удалось проверить constraint: %v", err)
	}
	
	// Если constraint не существует, создаем его
	if constraintCount == 0 {
		log.Printf("🔄 Добавляю UNIQUE constraint для (video_id, platform, format_id)...")
		
		// Создаем уникальный индекс
		uniqueIndexQuery := `CREATE UNIQUE INDEX IF NOT EXISTS idx_video_platform_format ON video_cache(video_id, platform, format_id)`
		_, err = db.Exec(uniqueIndexQuery)
		if err != nil {
			log.Printf("⚠️ Предупреждение: не удалось создать UNIQUE constraint: %v", err)
		} else {
			log.Printf("✅ UNIQUE constraint добавлен успешно")
		}
	}
	
	// Создаем индексы
	indexQueries := []string{
		`CREATE INDEX IF NOT EXISTS idx_video_id ON video_cache(video_id)`,
		`CREATE INDEX IF NOT EXISTS idx_platform ON video_cache(platform)`,
		`CREATE INDEX IF NOT EXISTS idx_format_id ON video_cache(format_id)`,
		`CREATE INDEX IF NOT EXISTS idx_download_count ON video_cache(download_count)`,
		`CREATE INDEX IF NOT EXISTS idx_last_download ON video_cache(last_download)`,
	}
	
	for _, indexQuery := range indexQueries {
		_, err = db.Exec(indexQuery)
		if err != nil {
			log.Printf("⚠️ Предупреждение: не удалось создать индекс: %v", err)
		}
	}
	
	return nil
}

// IsVideoCached проверяет, есть ли видео в кэше
func (cs *CacheService) IsVideoCached(videoID, platform, formatID string) (bool, *VideoCache, error) {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()
	
	query := `SELECT id, video_id, platform, url, title, download_count, last_download, file_size, file_path, format_id, resolution, created_at 
			  FROM video_cache WHERE video_id = ? AND platform = ? AND format_id = ?`
	
	var cache VideoCache
	err := cs.db.QueryRow(query, videoID, platform, formatID).Scan(
		&cache.ID, &cache.VideoID, &cache.Platform, &cache.URL, &cache.Title, &cache.DownloadCount,
		&cache.LastDownload, &cache.FileSize, &cache.FilePath, &cache.FormatID,
		&cache.Resolution, &cache.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, fmt.Errorf("ошибка проверки кэша: %v", err)
	}

	// Проверяем, существует ли файл
	if _, err := os.Stat(cache.FilePath); os.IsNotExist(err) {
		// Файл удален, удаляем запись из БД (с проверкой ошибки)
		cs.mutex.RUnlock() // Временно разблокируем для записи
		cs.mutex.Lock()
		if _, deleteErr := cs.db.Exec("DELETE FROM video_cache WHERE id = ?", cache.ID); deleteErr != nil {
			log.Printf("⚠️ Ошибка удаления записи из БД: %v", deleteErr)
		}
		cs.mutex.Unlock()
		cs.mutex.RLock() // Возвращаем блокировку чтения
		return false, nil, nil
	}

	return true, &cache, nil
}

// AddToCache добавляет видео в кэш
func (cs *CacheService) AddToCache(videoID, platform, url, title, formatID, resolution, filePath string, fileSize int64) error {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	
	// Проверяем размер кэша и очищаем если нужно
	if err := cs.ensureCacheSize(fileSize); err != nil {
		return fmt.Errorf("ошибка очистки кэша: %v", err)
	}

	// Сначала проверяем, существует ли запись
	existsQuery := `SELECT COUNT(*) FROM video_cache WHERE video_id = ? AND platform = ? AND format_id = ?`
	var count int
	err := cs.db.QueryRow(existsQuery, videoID, platform, formatID).Scan(&count)
	if err != nil {
		return fmt.Errorf("ошибка проверки существования записи: %v", err)
	}
	
	if count > 0 {
		// Обновляем существующую запись
		updateQuery := `
		UPDATE video_cache SET 
			url = ?, title = ?, file_size = ?, file_path = ?, resolution = ?, 
			last_download = CURRENT_TIMESTAMP, download_count = download_count + 1
		WHERE video_id = ? AND platform = ? AND format_id = ?
		`
		_, err = cs.db.Exec(updateQuery, url, title, fileSize, filePath, resolution, videoID, platform, formatID)
		if err != nil {
			return fmt.Errorf("ошибка обновления записи в кэше: %v", err)
		}
	} else {
		// Добавляем новую запись
		insertQuery := `
		INSERT INTO video_cache 
		(video_id, platform, url, title, download_count, last_download, file_size, file_path, format_id, resolution, created_at)
		VALUES (?, ?, ?, ?, 1, CURRENT_TIMESTAMP, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		`
		_, err = cs.db.Exec(insertQuery, videoID, platform, url, title, fileSize, filePath, formatID, resolution)
		if err != nil {
			return fmt.Errorf("ошибка добавления записи в кэш: %v", err)
		}
	}

	log.Printf("💾 Видео добавлено в кэш: %s (%s) - %s [%s]", videoID, resolution, formatID, platform)
	return nil
}

// IncrementDownloadCount увеличивает счетчик скачиваний
func (cs *CacheService) IncrementDownloadCount(videoID, platform, formatID string) error {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	
	query := `UPDATE video_cache SET download_count = download_count + 1, last_download = CURRENT_TIMESTAMP 
			  WHERE video_id = ? AND platform = ? AND format_id = ?`
	
	_, err := cs.db.Exec(query, videoID, platform, formatID)
	if err != nil {
		return fmt.Errorf("ошибка обновления счетчика: %v", err)
	}

	return nil
}

// GetPopularVideos возвращает популярные видео (5+ скачиваний)
func (cs *CacheService) GetPopularVideos() ([]VideoCache, error) {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()
	
	query := `SELECT id, video_id, platform, url, title, download_count, last_download, file_size, file_path, format_id, resolution, created_at 
			  FROM video_cache WHERE download_count >= 5 ORDER BY download_count DESC, last_download DESC`
	
	rows, err := cs.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения популярных видео: %v", err)
	}
	defer rows.Close()

	var videos []VideoCache
	for rows.Next() {
		var cache VideoCache
		err := rows.Scan(
			&cache.ID, &cache.VideoID, &cache.Platform, &cache.URL, &cache.Title, &cache.DownloadCount,
			&cache.LastDownload, &cache.FileSize, &cache.FilePath, &cache.FormatID,
			&cache.Resolution, &cache.CreatedAt,
		)
		if err != nil {
			log.Printf("⚠️ Ошибка сканирования строки: %v", err)
			continue
		}
		videos = append(videos, cache)
	}

	return videos, nil
}

// ensureCacheSize проверяет размер кэша и очищает старые файлы если нужно
func (cs *CacheService) ensureCacheSize(newFileSize int64) error {
	// Получаем текущий размер кэша
	var totalSize int64
	query := `SELECT COALESCE(SUM(file_size), 0) FROM video_cache`
	err := cs.db.QueryRow(query).Scan(&totalSize)
	if err != nil {
		return fmt.Errorf("ошибка подсчета размера кэша: %v", err)
	}

	// Если после добавления нового файла превысим лимит
	if totalSize+newFileSize > cs.maxCacheSize {
		log.Printf("⚠️ Кэш превышает лимит (%d GB), очищаю старые файлы", cs.maxCacheSize/(1024*1024*1024))
		
		// Удаляем старые файлы пока не освободим достаточно места
		query = `SELECT id, file_path, file_size FROM video_cache ORDER BY last_download ASC`
		rows, err := cs.db.Query(query)
		if err != nil {
			return fmt.Errorf("ошибка получения старых файлов: %v", err)
		}
		defer rows.Close()

		for rows.Next() {
			var id int64
			var filePath string
			var fileSize int64
			
			if err := rows.Scan(&id, &filePath, &fileSize); err != nil {
				continue
			}

			// Удаляем файл
			if err := os.Remove(filePath); err != nil {
				log.Printf("⚠️ Не удалось удалить файл %s: %v", filePath, err)
				continue
			}

			// Удаляем запись из БД (с проверкой ошибки)
			if _, deleteErr := cs.db.Exec("DELETE FROM video_cache WHERE id = ?", id); deleteErr != nil {
				log.Printf("⚠️ Ошибка удаления записи из БД: %v", deleteErr)
				continue
			}
			
			totalSize -= fileSize
			log.Printf("🗑️ Удален старый файл: %s (%d байт)", filePath, fileSize)

			// Проверяем, достаточно ли места
			if totalSize+newFileSize <= cs.maxCacheSize {
				break
			}
		}
	}

	return nil
}

// cleanupOldFiles очищает файлы старше 30 дней
func (cs *CacheService) cleanupOldFiles() error {
	query := `SELECT id, file_path FROM video_cache WHERE last_download < datetime('now', '-30 days')`
	rows, err := cs.db.Query(query)
	if err != nil {
		return fmt.Errorf("ошибка получения старых файлов: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var filePath string
		
		if err := rows.Scan(&id, &filePath); err != nil {
			continue
		}

		// Удаляем файл
		if err := os.Remove(filePath); err != nil {
			log.Printf("⚠️ Не удалось удалить старый файл %s: %v", filePath, err)
			continue
		}

		// Удаляем запись из БД (с проверкой ошибки)
		if _, deleteErr := cs.db.Exec("DELETE FROM video_cache WHERE id = ?", id); deleteErr != nil {
			log.Printf("⚠️ Ошибка удаления записи из БД: %v", deleteErr)
		}
		log.Printf("🗑️ Удален старый файл: %s", filePath)
	}

	return nil
}

// Close закрывает соединение с БД
func (cs *CacheService) Close() error {
	return cs.db.Close()
}
