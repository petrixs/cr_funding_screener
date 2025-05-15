module github.com/petrixs/cr_funding_screener

go 1.24.2

require (
	github.com/go-telegram-bot-api/telegram-bot-api/v5 v5.5.1
	github.com/joho/godotenv v1.5.1
	github.com/petrixs/cr-exchanges v1.0.0
	github.com/petrixs/cr-transport-bus v1.0.0
	github.com/rabbitmq/amqp091-go v1.10.0
)

require google.golang.org/protobuf v1.36.6 // indirect

replace github.com/petrixs/cr-exchanges => ./packages/exchanges

replace github.com/petrixs/cr-transport-bus => ./packages/transport-bus
