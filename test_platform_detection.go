package main

import (
	"fmt"
	"youtubeBot/services"
)

func main() {
	// Создаем детектор платформ
	detector := services.NewPlatformDetector()
	
	// Тестируем различные URL
	testURLs := []string{
		"https://youtu.be/_AbFXuGDRTs?feature=shared",
		"https://youtu.be/A4sMjYyN7FM?si=id-aAyQAoef6HvKv",
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		"https://youtube.com/shorts/cU8Vd8eTKHs?si=oZRiNp2-dj_tCo0Y",
		"https://www.tiktok.com/@user/video/123456789",
		"https://instagram.com/p/ABC123",
		"https://vk.com/video123456_789",
		"https://twitter.com/user/status/123456789",
		"https://facebook.com/user/videos/123456789",
		"https://example.com/invalid",
	}
	
	fmt.Println("🧪 Тестирование детекции платформ:")
	fmt.Println("=" * 50)
	
	for _, url := range testURLs {
		info := detector.DetectPlatform(url)
		status := "✅"
		if !info.Supported {
			status = "❌"
		}
		
		fmt.Printf("%s %s %s\n", status, info.Icon, info.DisplayName)
		fmt.Printf("   URL: %s\n", url)
		fmt.Printf("   Video ID: %s\n", info.VideoID)
		fmt.Printf("   Поддерживается: %v\n", info.Supported)
		fmt.Println()
	}
}
