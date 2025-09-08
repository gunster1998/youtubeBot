module youtubeBot

go 1.25.0

require (
	github.com/go-telegram-bot-api/telegram-bot-api/v5 v5.5.1
	github.com/mattn/go-sqlite3 v1.14.17
	youtubeBot/config v0.0.0-00010101000000-000000000000
	youtubeBot/services v0.0.0-00010101000000-000000000000
	youtubeBot/handlers v0.0.0-00010101000000-000000000000
	youtubeBot/utils v0.0.0-00010101000000-000000000000
)

replace youtubeBot/config => ./config
replace youtubeBot/services => ./services
replace youtubeBot/handlers => ./handlers
replace youtubeBot/utils => ./utils
