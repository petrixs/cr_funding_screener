package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/petrixs/cr_funding_screener/internal/bot"
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
