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
	PlatformTikTok      PlatformType = "tiktok"
	PlatformInstagram   PlatformType = "instagram"
	PlatformVK          PlatformType = "vkontakte"
	PlatformTwitter     PlatformType = "twitter"
	PlatformFacebook    PlatformType = "facebook"
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
			},
			PlatformYouTubeShorts: {
				`youtube\.com/shorts/([a-zA-Z0-9_-]{11})`,
			},
			PlatformTikTok: {
				`tiktok\.com/@[^/]+/video/(\d+)`,
				`vm\.tiktok\.com/([a-zA-Z0-9]+)`,
				`tiktok\.com/t/([a-zA-Z0-9]+)`,
			},
			PlatformInstagram: {
				`instagram\.com/p/([a-zA-Z0-9_-]+)`,
				`instagram\.com/reel/([a-zA-Z0-9_-]+)`,
				`instagram\.com/tv/([a-zA-Z0-9_-]+)`,
			},
			PlatformVK: {
				`vk\.com/video(-?\d+_\d+)`,
				`vk\.com/videos(-?\d+_\d+)`,
			},
			PlatformTwitter: {
				`twitter\.com/\w+/status/(\d+)`,
				`x\.com/\w+/status/(\d+)`,
			},
			PlatformFacebook: {
				`facebook\.com/\w+/videos/(\d+)`,
				`fb\.watch/([a-zA-Z0-9_-]+)`,
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
		PlatformTikTok:        "TikTok",
		PlatformInstagram:     "Instagram",
		PlatformVK:            "VKontakte",
		PlatformTwitter:       "Twitter/X",
		PlatformFacebook:      "Facebook",
		PlatformUnknown:       "Неизвестная платформа",
	}
	return names[platformType]
}

// getIcon возвращает иконку платформы
func (pd *PlatformDetector) getIcon(platformType PlatformType) string {
	icons := map[PlatformType]string{
		PlatformYouTube:       "🎬",
		PlatformYouTubeShorts: "🎬",
		PlatformTikTok:        "🎵",
		PlatformInstagram:     "📸",
		PlatformVK:            "🔵",
		PlatformTwitter:       "🐦",
		PlatformFacebook:      "📘",
		PlatformUnknown:       "❓",
	}
	return icons[platformType]
}

// isSupported проверяет, поддерживается ли платформа
func (pd *PlatformDetector) isSupported(platformType PlatformType) bool {
	supported := map[PlatformType]bool{
		PlatformYouTube:       true,
		PlatformYouTubeShorts: true,
		PlatformTikTok:        true,
		PlatformInstagram:     true,
		PlatformVK:            true,
		PlatformTwitter:       true,
		PlatformFacebook:      true,
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
		args = append(args, "--format", "best[ext=mp4]/best")
		
	case PlatformTikTok:
		// Специальные аргументы для TikTok
		args = append(args, 
			"--format", "best",
			"--extractor-args", "tiktok:webpage_url_basename=video",
		)
		
	case PlatformInstagram:
		// Специальные аргументы для Instagram
		args = append(args,
			"--format", "best",
			"--extractor-args", "instagram:webpage_url_basename=reel",
		)
		
	case PlatformVK:
		// Специальные аргументы для VK
		args = append(args,
			"--format", "best",
			"--extractor-args", "vkontakte:webpage_url_basename=video",
		)
		
	case PlatformTwitter:
		// Специальные аргументы для Twitter
		args = append(args,
			"--format", "best",
			"--extractor-args", "twitter:webpage_url_basename=status",
		)
		
	case PlatformFacebook:
		// Специальные аргументы для Facebook
		args = append(args,
			"--format", "best",
			"--extractor-args", "facebook:webpage_url_basename=videos",
		)
		
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
		PlatformTikTok:        fmt.Sprintf("TikTok Video %s", videoID),
		PlatformInstagram:     fmt.Sprintf("Instagram Reel %s", videoID),
		PlatformVK:            fmt.Sprintf("VK Video %s", videoID),
		PlatformTwitter:       fmt.Sprintf("Twitter Video %s", videoID),
		PlatformFacebook:      fmt.Sprintf("Facebook Video %s", videoID),
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
