package main

import (
	"fmt"
	"youtubeBot/config"
)

func main() {
	fmt.Println("Тестирование импортов...")
	cfg, err := config.Load("config.env")
	if err != nil {
		fmt.Printf("Ошибка загрузки конфигурации: %v\n", err)
	} else {
		fmt.Printf("Конфигурация загружена: %s\n", cfg.TelegramAPI)
	}
}


