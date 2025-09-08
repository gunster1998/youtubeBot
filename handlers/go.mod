module youtubeBot/handlers

go 1.25.0

require (
	github.com/go-telegram-bot-api/telegram-bot-api/v5 v5.5.1
	youtubeBot/services v0.0.0-00010101000000-000000000000
	youtubeBot/utils v0.0.0-00010101000000-000000000000
)

replace youtubeBot/services => ../services
replace youtubeBot/utils => ../utils


