package main

import (
	"context"
	"log"
	"os"

	"github.com/joho/godotenv"
	exchanges "github.com/petrixs/cr-exchanges"
	"github.com/petrixs/cr-transport-bus/proto"
	rabbit "github.com/petrixs/cr-transport-bus/rabbit"
	"github.com/petrixs/cr_funding_screener/internal/bot"
	amqp091 "github.com/rabbitmq/amqp091-go"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Запуск бота...")

	if err := godotenv.Load(); err != nil {
		log.Fatal("Ошибка загрузки .env файла:", err)
	}

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN не установлен")
	}

	amqpURL := os.Getenv("AMQP_URL")
	if amqpURL == "" {
		log.Fatal("AMQP_URL не установлен")
	}
	conn, err := amqp091.Dial(amqpURL)
	if err != nil {
		log.Fatalf("Ошибка подключения к RabbitMQ: %v", err)
	}
	defer conn.Close()

	fundingChan := make(chan *proto.FundingRate, 100)

	// Горутина для отправки ставок в RabbitMQ
	go func() {
		for rate := range fundingChan {
			err := rabbit.PublishProtoJSONWithTTL(
				context.Background(), conn, "funding_rates", rate, 0, // TTL=0 бессрочно
			)
			if err != nil {
				log.Printf("Ошибка отправки в RabbitMQ: %v", err)
			}
		}
	}()

	log.Println("Инициализация бирж...")
	binance := exchanges.NewBinance()
	bybit := exchanges.NewBybit()
	htx := exchanges.NewHTX()
	okx := exchanges.NewOKX()
	gate := exchanges.NewGate()
	kucoin := exchanges.NewKuCoin()
	bingx := exchanges.NewBingX()
	log.Println("Биржи инициализированы")

	log.Println("Создание бота...")
	telegramBot := bot.NewBot(botToken, []exchanges.Exchange{
		binance,
		bybit,
		htx,
		okx,
		gate,
		kucoin,
		bingx,
	})
	log.Println("Бот создан")

	log.Println("Запуск бота...")
	if err := telegramBot.Start(); err != nil {
		log.Fatal("Ошибка запуска бота:", err)
	}
}
