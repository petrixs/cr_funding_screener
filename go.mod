module github.com/petrixs/cr_funding_screener

go 1.21

require (
	github.com/go-telegram-bot-api/telegram-bot-api/v5 v5.5.1
	github.com/joho/godotenv v1.5.1
)

replace github.com/petrixs/cr-exchanges => ./packages/exchanges