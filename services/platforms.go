package services

import (
	"fmt"
	"log"
	"regexp"
	"strings"
)

// PlatformType представляет тип платформы
type PlatformType string

const (
	PlatformYouTube     PlatformType = "youtube"
	PlatformYouTubeShorts PlatformType = "youtube_shorts"
	PlatformUnknown     PlatformType = "unknown"
)

// PlatformInfo содержит информацию о платформе
type PlatformInfo struct {
	Type        PlatformType
	VideoID     string
	DisplayName string
	Icon        string
	Supported   bool
}

// PlatformDetector определяет платформу по URL
type PlatformDetector struct {
	patterns map[PlatformType][]string
}

// NewPlatformDetector создает новый детектор платформ
func NewPlatformDetector() *PlatformDetector {
	return &PlatformDetector{
		patterns: map[PlatformType][]string{
			PlatformYouTube: {
				`youtube\.com/watch\?v=([a-zA-Z0-9_-]{11})`,
				`youtube\.com/embed/([a-zA-Z0-9_-]{11})`,
				`youtube\.com/v/([a-zA-Z0-9_-]{11})`,
				`youtu\.be/([a-zA-Z0-9_-]{11})`,
			},
			PlatformYouTubeShorts: {
				`youtube\.com/shorts/([a-zA-Z0-9_-]{11})`,
			},
		},
	}
}

// DetectPlatform определяет платформу по URL
func (pd *PlatformDetector) DetectPlatform(url string) *PlatformInfo {
	url = strings.TrimSpace(url)
	
	// Проверяем каждую платформу
	for platformType, patterns := range pd.patterns {
		for _, pattern := range patterns {
			re := regexp.MustCompile(pattern)
			matches := re.FindStringSubmatch(url)
			if len(matches) > 1 {
				videoID := matches[1]
				return &PlatformInfo{
					Type:        platformType,
					VideoID:     videoID,
					DisplayName: pd.getDisplayName(platformType),
					Icon:        pd.getIcon(platformType),
					Supported:   pd.isSupported(platformType),
				}
			}
		}
	}
	
	return &PlatformInfo{
		Type:        PlatformUnknown,
		VideoID:     "",
		DisplayName: "Неизвестная платформа",
		Icon:        "❓",
		Supported:   false,
	}
}

// getDisplayName возвращает отображаемое имя платформы
func (pd *PlatformDetector) getDisplayName(platformType PlatformType) string {
	names := map[PlatformType]string{
		PlatformYouTube:       "YouTube",
		PlatformYouTubeShorts: "YouTube Shorts",
		PlatformUnknown:       "Неизвестная платформа",
	}
	return names[platformType]
}

// getIcon возвращает иконку платформы
func (pd *PlatformDetector) getIcon(platformType PlatformType) string {
	icons := map[PlatformType]string{
		PlatformYouTube:       "🎬",
		PlatformYouTubeShorts: "🎬",
		PlatformUnknown:       "❓",
	}
	return icons[platformType]
}

// isSupported проверяет, поддерживается ли платформа
func (pd *PlatformDetector) isSupported(platformType PlatformType) bool {
	supported := map[PlatformType]bool{
		PlatformYouTube:       true,
		PlatformYouTubeShorts: true,
		PlatformUnknown:       false,
	}
	return supported[platformType]
}

// IsValidURL проверяет, является ли URL валидным для любой поддерживаемой платформы
func (pd *PlatformDetector) IsValidURL(url string) bool {
	info := pd.DetectPlatform(url)
	return info.Supported && info.VideoID != ""
}

// GetSupportedPlatforms возвращает список поддерживаемых платформ
func (pd *PlatformDetector) GetSupportedPlatforms() []PlatformInfo {
	var platforms []PlatformInfo
	for platformType := range pd.patterns {
		if pd.isSupported(platformType) {
			platforms = append(platforms, PlatformInfo{
				Type:        platformType,
				VideoID:     "",
				DisplayName: pd.getDisplayName(platformType),
				Icon:        pd.getIcon(platformType),
				Supported:   true,
			})
		}
	}
	return platforms
}

// GetYtDlpArgs возвращает аргументы yt-dlp для конкретной платформы
func (pd *PlatformDetector) GetYtDlpArgs(platformType PlatformType) []string {
	args := []string{
		"--no-playlist",
		"--no-check-certificates",
		"--max-filesize", "2G",
		"--socket-timeout", "60",
		"--retries", "5",
	}
	
	switch platformType {
	case PlatformYouTube, PlatformYouTubeShorts:
		// Стандартные аргументы для YouTube
		args = append(args, "--format", "best[ext=mp4]+bestaudio[ext=m4a]/best[ext=mp4]+bestaudio/best")
		
	default:
		// Универсальные аргументы
		args = append(args, "--format", "best")
	}
	
	return args
}

// GetVideoTitle возвращает заголовок видео для кэша
func (pd *PlatformDetector) GetVideoTitle(platformType PlatformType, videoID string) string {
	titles := map[PlatformType]string{
		PlatformYouTube:       fmt.Sprintf("YouTube Video %s", videoID),
		PlatformYouTubeShorts: fmt.Sprintf("YouTube Short %s", videoID),
		PlatformUnknown:       fmt.Sprintf("Video %s", videoID),
	}
	return titles[platformType]
}

// LogPlatformInfo логирует информацию о платформе
func (pd *PlatformDetector) LogPlatformInfo(info *PlatformInfo, url string) {
	log.Printf("🔍 Обнаружена платформа: %s %s", info.Icon, info.DisplayName)
	log.Printf("   URL: %s", url)
	log.Printf("   Video ID: %s", info.VideoID)
	log.Printf("   Поддерживается: %v", info.Supported)
}
