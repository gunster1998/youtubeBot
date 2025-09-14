module youtubeBot

go 1.22

require (
	youtubeBot/config v0.0.0-00010101000000-000000000000
	youtubeBot/services v0.0.0-00010101000000-000000000000
)

require (
	github.com/mattn/go-sqlite3 v1.14.17 // indirect
	golang.org/x/net v0.30.0 // indirect
	youtubeBot/utils v0.0.0-00010101000000-000000000000 // indirect
)

replace youtubeBot/config => ./config

replace youtubeBot/services => ./services

replace youtubeBot/handlers => ./handlers

replace youtubeBot/utils => ./utils
