package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/joho/godotenv"
	exchanges "github.com/petrixs/cr-exchanges"
	"github.com/petrixs/cr-transport-bus/proto"
	rabbit "github.com/petrixs/cr-transport-bus/rabbit"
	"github.com/petrixs/cr_funding_screener/internal/bot"
	"github.com/petrixs/cr_funding_screener/internal/logger"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Запуск бота...")

	// Обработка сигналов для корректного завершения
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Закрываем логгеры при завершении
	defer logger.CloseAll()

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

	queueName := os.Getenv("FUNDING_QUEUE")
	if queueName == "" {
		queueName = "funding_rates"
	}

	fundingTTL := 0
	if ttlStr := os.Getenv("FUNDING_TTL_MS"); ttlStr != "" {
		if v, err := strconv.Atoi(ttlStr); err == nil {
			fundingTTL = v
		}
	}

	// Создаем клиент RabbitMQ с автоматическим переподключением
	rabbitClient, err := rabbit.NewRabbitMQClient(amqpURL)
	if err != nil {
		log.Fatalf("Не удалось создать клиент RabbitMQ: %v", err)
	}
	defer rabbitClient.Close()

	fundingChan := make(chan *proto.FundingRate, 10000)

	// Горутина для отправки ставок в RabbitMQ
	go func() {
		for rate := range fundingChan {
			// Логируем в файл конкретной биржи
			exchangeLogger := logger.GetExchangeLogger(rate.Exchange)
			exchangeLogger.Printf("Публикую в RabbitMQ: %+v", rate)

			log.Printf("Публикую в RabbitMQ: %+v", rate)
			err := rabbitClient.PublishProtoJSONWithTTL(
				context.Background(), queueName, rate, fundingTTL,
			)
			if err != nil {
				log.Printf("Ошибка отправки в RabbitMQ: %v", err)
				if exchangeLogger != nil {
					exchangeLogger.Printf("Ошибка отправки в RabbitMQ: %v", err)
				}
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
	mexc := exchanges.NewMEXC()
	hyperliquid := exchanges.NewHyperliquid()
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
		mexc,
		hyperliquid,
	}, fundingChan)
	log.Println("Бот создан")

	log.Println("Запуск бота...")
	go func() {
		if err := telegramBot.Start(); err != nil {
			log.Fatal("Ошибка запуска бота:", err)
		}
	}()

	// Ожидание сигнала завершения
	<-sigChan
	log.Println("Получен сигнал завершения, закрываем приложение...")
}
