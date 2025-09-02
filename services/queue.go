package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DownloadJob представляет задачу загрузки
type DownloadJob struct {
	ID        string    // Уникальный ID задачи
	UserID    int64     // ID пользователя
	ChatID    int64     // ID чата
	VideoURL  string    // URL видео
	FormatID  string    // ID формата
	Priority  int       // Приоритет (1-10, где 10 - высший)
	CreatedAt time.Time // Время создания
	Status    JobStatus // Статус задачи
	Error     error     // Ошибка если есть
	Result    string    // Результат (путь к файлу)
}

// JobStatus представляет статус задачи
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"   // В очереди
	JobStatusProcessing JobStatus = "processing" // Обрабатывается
	JobStatusCompleted JobStatus = "completed" // Завершена
	JobStatusFailed    JobStatus = "failed"    // Ошибка
	JobStatusCancelled JobStatus = "cancelled" // Отменена
)

// JobResult представляет результат выполнения задачи
type JobResult struct {
	JobID   string
	Status  JobStatus
	Result  string
	Error   error
}

// DownloadQueue управляет очередью загрузок
type DownloadQueue struct {
	jobs           chan DownloadJob
	results        chan JobResult
	workers        int
	activeJobs     map[string]*DownloadJob
	activeJobsMux  sync.RWMutex
	jobCounter     int64
	jobCounterMux  sync.Mutex
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	youtubeService *YouTubeService
	cacheService   *CacheService
}

// NewDownloadQueue создает новую очередь загрузок
func NewDownloadQueue(workers int, youtubeService *YouTubeService, cacheService *CacheService) *DownloadQueue {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &DownloadQueue{
		jobs:           make(chan DownloadJob, 1000), // Буфер на 1000 задач
		results:        make(chan JobResult, 1000),
		workers:        workers,
		activeJobs:     make(map[string]*DownloadJob),
		ctx:            ctx,
		cancel:         cancel,
		youtubeService: youtubeService,
		cacheService:   cacheService,
	}
}

// Start запускает воркеры очереди
func (q *DownloadQueue) Start() {
	log.Printf("🚀 Запуск очереди загрузок с %d воркерами", q.workers)
	
	// Запускаем воркеры
	for i := 0; i < q.workers; i++ {
		q.wg.Add(1)
		go q.worker(i)
	}
	
	// Запускаем обработчик результатов
	q.wg.Add(1)
	go q.resultHandler()
	
	log.Printf("✅ Очередь загрузок запущена")
}

// Stop останавливает очередь
func (q *DownloadQueue) Stop() {
	log.Printf("🛑 Остановка очереди загрузок...")
	q.cancel()
	close(q.jobs)
	close(q.results)
	q.wg.Wait()
	log.Printf("✅ Очередь загрузок остановлена")
}

// AddJob добавляет задачу в очередь
func (q *DownloadQueue) AddJob(userID, chatID int64, videoURL, formatID string, priority int) (string, error) {
	q.jobCounterMux.Lock()
	q.jobCounter++
	jobID := fmt.Sprintf("job_%d_%d", time.Now().Unix(), q.jobCounter)
	q.jobCounterMux.Unlock()
	
	job := DownloadJob{
		ID:        jobID,
		UserID:    userID,
		ChatID:    chatID,
		VideoURL:  videoURL,
		FormatID:  formatID,
		Priority:  priority,
		CreatedAt: time.Now(),
		Status:    JobStatusPending,
	}
	
	select {
	case q.jobs <- job:
		log.Printf("📝 Задача добавлена в очередь: %s (пользователь: %d, приоритет: %d)", 
			jobID, userID, priority)
		return jobID, nil
	case <-q.ctx.Done():
		return "", fmt.Errorf("очередь остановлена")
	default:
		return "", fmt.Errorf("очередь переполнена")
	}
}

// GetJobStatus возвращает статус задачи
func (q *DownloadQueue) GetJobStatus(jobID string) (*DownloadJob, bool) {
	q.activeJobsMux.RLock()
	defer q.activeJobsMux.RUnlock()
	
	job, exists := q.activeJobs[jobID]
	return job, exists
}

// GetUserJobs возвращает все задачи пользователя
func (q *DownloadQueue) GetUserJobs(userID int64) []*DownloadJob {
	q.activeJobsMux.RLock()
	defer q.activeJobsMux.RUnlock()
	
	var userJobs []*DownloadJob
	for _, job := range q.activeJobs {
		if job.UserID == userID {
			userJobs = append(userJobs, job)
		}
	}
	
	return userJobs
}

// CancelJob отменяет задачу
func (q *DownloadQueue) CancelJob(jobID string) error {
	q.activeJobsMux.Lock()
	defer q.activeJobsMux.Unlock()
	
	job, exists := q.activeJobs[jobID]
	if !exists {
		return fmt.Errorf("задача не найдена")
	}
	
	if job.Status == JobStatusCompleted || job.Status == JobStatusFailed {
		return fmt.Errorf("задача уже завершена")
	}
	
	job.Status = JobStatusCancelled
	log.Printf("❌ Задача отменена: %s", jobID)
	return nil
}

// worker обрабатывает задачи из очереди
func (q *DownloadQueue) worker(workerID int) {
	defer q.wg.Done()
	
	log.Printf("👷 Воркер %d запущен", workerID)
	
	for {
		select {
		case job := <-q.jobs:
			if job.Status == JobStatusCancelled {
				continue
			}
			
			// Добавляем задачу в активные
			q.activeJobsMux.Lock()
			q.activeJobs[job.ID] = &job
			q.activeJobsMux.Unlock()
			
			// Обрабатываем задачу
			q.processJob(workerID, &job)
			
			// НЕ удаляем задачу из активных сразу - пусть resultHandler это сделает
			// q.activeJobsMux.Lock()
			// delete(q.activeJobs, job.ID)
			// q.activeJobsMux.Unlock()
			
		case <-q.ctx.Done():
			log.Printf("👷 Воркер %d остановлен", workerID)
			return
		}
	}
}

// processJob обрабатывает конкретную задачу
func (q *DownloadQueue) processJob(workerID int, job *DownloadJob) {
	log.Printf("🔄 Воркер %d обрабатывает задачу %s: %s", workerID, job.ID, job.VideoURL)
	
	// Обновляем статус
	job.Status = JobStatusProcessing
	
	// Проверяем кэш
	videoID := extractVideoID(job.VideoURL)
	if videoID != "" {
		if isCached, cachedVideo, err := q.cacheService.IsVideoCached(videoID, job.FormatID); err == nil && isCached {
			// Видео в кэше - отправляем результат
			log.Printf("⚡ Задача %s: видео найдено в кэше", job.ID)
			
			// Увеличиваем счетчик скачиваний
			q.cacheService.IncrementDownloadCount(videoID, job.FormatID)
			
			// Отправляем результат в канал
			select {
			case q.results <- JobResult{
				JobID:  job.ID,
				Status: JobStatusCompleted,
				Result: cachedVideo.FilePath,
			}:
				log.Printf("✅ Задача %s: результат кэша отправлен в канал", job.ID)
			case <-q.ctx.Done():
				log.Printf("⚠️ Задача %s: контекст отменен при отправке результата кэша", job.ID)
				return
			}
			return
		}
	}
	
	// Скачиваем видео
	log.Printf("📥 Задача %s: скачиваю видео...", job.ID)
	videoPath, err := q.youtubeService.DownloadVideoWithFormat(job.VideoURL, job.FormatID)
	if err != nil {
		log.Printf("❌ Задача %s: ошибка загрузки: %v", job.ID, err)
		q.results <- JobResult{
			JobID:  job.ID,
			Status: JobStatusFailed,
			Error:  err,
		}
		return
	}
	
	// Сохраняем в кэш (только для видео, не для аудио)
	if videoID != "" && !isAudioFile(videoPath) {
		if fileInfo, err := os.Stat(videoPath); err == nil {
			// Находим разрешение для формата
			formats, _ := q.youtubeService.GetVideoFormats(job.VideoURL)
			var resolution string
			for _, f := range formats {
				if f.ID == job.FormatID {
					resolution = f.Resolution
					break
				}
			}
			
			// Добавляем в кэш
			if err := q.cacheService.AddToCache(videoID, job.VideoURL, "YouTube Video", job.FormatID, resolution, videoPath, fileInfo.Size()); err != nil {
				log.Printf("⚠️ Задача %s: не удалось добавить в кэш: %v", job.ID, err)
			}
		}
	}
	
	log.Printf("✅ Задача %s: загрузка завершена: %s", job.ID, videoPath)
	q.results <- JobResult{
		JobID:  job.ID,
		Status: JobStatusCompleted,
		Result: videoPath,
	}
}

// resultHandler обрабатывает результаты выполнения задач
func (q *DownloadQueue) resultHandler() {
	defer q.wg.Done()
	
	log.Printf("📊 Обработчик результатов запущен")
	
	for {
		select {
		case result := <-q.results:
			log.Printf("📋 Результат задачи %s: %s", result.JobID, result.Status)
			
			// Обновляем статус задачи в активных задачах
			q.activeJobsMux.Lock()
			if job, exists := q.activeJobs[result.JobID]; exists {
				job.Status = result.Status
				job.Result = result.Result
				job.Error = result.Error
				log.Printf("✅ Обновлен статус задачи %s: %s", result.JobID, result.Status)
				
				// Даем время monitorJob обработать завершенную задачу
				go func(jobID string) {
					time.Sleep(5 * time.Second) // Ждем 5 секунд
					q.activeJobsMux.Lock()
					delete(q.activeJobs, jobID)
					log.Printf("🗑️ Задача %s удалена из активных после обработки результата", jobID)
					q.activeJobsMux.Unlock()
				}(result.JobID)
			} else {
				log.Printf("⚠️ Задача %s не найдена в активных при обработке результата", result.JobID)
			}
			q.activeJobsMux.Unlock()
			
		case <-q.ctx.Done():
			log.Printf("📊 Обработчик результатов остановлен")
			return
		}
	}
}

// GetQueueStats возвращает статистику очереди
func (q *DownloadQueue) GetQueueStats() map[string]interface{} {
	q.activeJobsMux.RLock()
	defer q.activeJobsMux.RUnlock()
	
	stats := map[string]interface{}{
		"workers":        q.workers,
		"active_jobs":    len(q.activeJobs),
		"queue_length":   len(q.jobs),
		"results_buffer": len(q.results),
	}
	
	// Подсчитываем задачи по статусам
	statusCounts := make(map[JobStatus]int)
	for _, job := range q.activeJobs {
		statusCounts[job.Status]++
	}
	stats["status_counts"] = statusCounts
	
	return stats
}

// isAudioFile проверяет, является ли файл аудио
func isAudioFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	return ext == ".mp3" || ext == ".m4a" || ext == ".webm" || ext == ".ogg"
}