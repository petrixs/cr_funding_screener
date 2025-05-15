package bot

import (
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"encoding/json"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/petrixs/cr-exchanges"
)

type Bot struct {
	bot       *tgbotapi.BotAPI
	exchanges []exchanges.Exchange
	cache     *exchanges.RatesCache
}

var (
	subscribers     = make(map[int64]struct{})
	subscribersLock sync.Mutex

	userThresholds     = make(map[int64]float64)
	userThresholdsLock sync.Mutex

	settingsFile = "settings.json"
)

// Структура для хранения всех настроек
type Settings struct {
	Subscribers []int64           `json:"subscribers"`
	Thresholds  map[int64]float64 `json:"thresholds"`
}

// loadSettings загружает все настройки из файла
func loadSettings() {
	file, err := os.Open(settingsFile)
	if err != nil {
		log.Printf("Не удалось открыть файл настроек: %v", err)
		return
	}
	defer file.Close()

	var settings Settings
	if err := json.NewDecoder(file).Decode(&settings); err == nil {
		subscribersLock.Lock()
		for _, id := range settings.Subscribers {
			subscribers[id] = struct{}{}
		}
		subscribersLock.Unlock()
		log.Printf("Загружено подписчиков: %d", len(settings.Subscribers))

		userThresholdsLock.Lock()
		userThresholds = settings.Thresholds
		userThresholdsLock.Unlock()
		log.Printf("Загружено порогов: %d", len(settings.Thresholds))
	} else {
		log.Printf("Ошибка декодирования файла настроек: %v", err)
	}
}

// saveSettings сохраняет все настройки в файл
func saveSettings() {
	subscribersLock.Lock()
	userThresholdsLock.Lock()

	subscribers_list := make([]int64, 0, len(subscribers))
	for id := range subscribers {
		subscribers_list = append(subscribers_list, id)
	}

	settings := Settings{
		Subscribers: subscribers_list,
		Thresholds:  userThresholds,
	}

	subscribersLock.Unlock()
	userThresholdsLock.Unlock()

	file, err := os.Create(settingsFile)
	if err != nil {
		log.Printf("Ошибка создания файла настроек: %v", err)
		return
	}
	defer file.Close()

	if err := json.NewEncoder(file).Encode(settings); err != nil {
		log.Printf("Ошибка сохранения настроек: %v", err)
	}
}

// Получить порог для пользователя
func getUserThreshold(userID int64) (float64, string) {
	userThresholdsLock.Lock()
	defer userThresholdsLock.Unlock()
	if t, ok := userThresholds[userID]; ok {
		return t, "user"
	}
	// Если не задан, возвращаем глобальный по умолчанию
	return getDefaultThreshold(), "default"
}

// Установить порог для пользователя
func setUserThreshold(userID int64, threshold float64) {
	userThresholdsLock.Lock()
	userThresholds[userID] = threshold
	userThresholdsLock.Unlock()
	saveSettings()
}

// Получить глобальный порог из .env
func getDefaultThreshold() float64 {
	val := os.Getenv("DEFAULT_FUNDING_THRESHOLD")
	if val == "" {
		return 0.001 // 0.1% по умолчанию
	}

	// Убираем знак процента, если он есть
	val = strings.TrimSuffix(val, "%")

	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0.001
	}
	return f
}

func NewBot(token string, exs []exchanges.Exchange) *Bot {
	log.Println("Создание нового экземпляра бота...")
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatal("Ошибка создания бота:", err)
	}

	log.Printf("Бот создан: @%s", bot.Self.UserName)
	return &Bot{
		bot:       bot,
		exchanges: exs,
		cache:     exchanges.GetGlobalCache(),
	}
}

func (b *Bot) Start() error {
	log.Println("Запуск бота...")
	loadSettings()

	// Запускаем горутину для обновления кэша ставок
	go b.startRatesUpdateLoop()

	// Запускаем горутину для рассылки уведомлений
	go b.startBroadcastLoop()

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.bot.GetUpdatesChan(u)
	for update := range updates {
		if update.Message == nil {
			continue
		}

		go b.handleMessage(update.Message)
	}

	return nil
}

func (b *Bot) startRatesUpdateLoop() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for {
		b.updateAllRates()
		<-ticker.C
	}
}

func (b *Bot) updateAllRates() {
	var wg sync.WaitGroup
	for _, ex := range b.exchanges {
		wg.Add(1)
		go func(exchange exchanges.Exchange) {
			defer wg.Done()
			if err := b.cache.UpdateRates(exchange); err != nil {
				log.Printf("Ошибка обновления ставок для %s: %v", exchange.GetName(), err)
			}
		}(ex)
	}
	wg.Wait()
}

func (b *Bot) handleRates(msg *tgbotapi.Message) {
	rates := b.cache.GetAllRates()
	if len(rates) == 0 {
		b.sendLongMessage(msg.Chat.ID, "Нет доступных ставок фандинга")
		return
	}

	threshold, _ := getUserThreshold(msg.Chat.ID)
	formattedRates := formatRates(rates, threshold)
	b.sendLongMessage(msg.Chat.ID, formattedRates)
}

func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	if msg.Command() == "start" {
		go b.handleStart(msg)
	} else if msg.Command() == "rates" {
		go b.handleRates(msg)
	} else if msg.Command() == "subscribe" {
		go b.handleSubscribe(msg)
	} else if msg.Command() == "unsubscribe" {
		go b.handleUnsubscribe(msg)
	} else if msg.Command() == "threshold" {
		go b.handleThreshold(msg)
	}
}

func (b *Bot) handleSubscribe(msg *tgbotapi.Message) {
	log.Printf("Обработка команды subscribe от пользователя %s", msg.From.UserName)
	subscribersLock.Lock()
	subscribers[msg.Chat.ID] = struct{}{}
	subscribersLock.Unlock()
	// Устанавливаем порог по умолчанию для нового пользователя
	setUserThreshold(msg.Chat.ID, getDefaultThreshold())
	saveSettings()
	b.sendLongMessage(msg.Chat.ID, "Вы успешно подписались на уведомления!")
	// Сразу отправляем ставки после подписки
	go b.sendCurrentRatesToUser(msg.Chat.ID)
}

// sendCurrentRatesToUser отправляет актуальные ставки фандинга пользователю
func (b *Bot) sendCurrentRatesToUser(chatID int64) {
	threshold, _ := getUserThreshold(chatID)
	for _, exchange := range b.exchanges {
		rates, err := exchange.GetFundingRates()
		if err != nil {
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Ошибка получения данных с %s: %v", exchange.GetName(), err))
			msg.ParseMode = "MarkdownV2"
			if _, err := b.bot.Send(msg); err != nil {
				log.Printf("Ошибка отправки сообщения: %v", err)
			}
			continue
		}
		sort.SliceStable(rates, func(i, j int) bool {
			return math.Abs(rates[i].Rate) > math.Abs(rates[j].Rate)
		})
		var exchangeRates []string
		for _, rate := range rates {
			if math.Abs(rate.Rate) >= threshold {
				log.Printf("rate=%.6f, threshold=%.6f, проходит фильтр", rate.Rate, threshold)
				paymentTime := rate.NextFunding
				if paymentTime != "Неизвестно" {
					if t, err := time.Parse(time.RFC3339, rate.NextFunding); err == nil {
						if exchange.GetName() == "HTX" {
							paymentTime = t.Format("02.01.2006 15:04")
						} else {
							paymentTime = t.Format("02.01.2006 15:04 MST")
						}
					}
				}
				payDirection := ""
				payEmoji := ""
				if rate.Rate > 0 {
					payDirection = "Long → Short"
					payEmoji = "⬆️"
				} else if rate.Rate < 0 {
					payDirection = "Short → Long"
					payEmoji = "⬇️"
				}
				line := fmt.Sprintf("%-12s  %+8.4f%%  %s  %-12s  (выплата: %s)",
					rate.Symbol, rate.Rate*100, payEmoji, payDirection, paymentTime)
				exchangeRates = append(exchangeRates, line)
			}
		}
		if len(exchangeRates) > 0 {
			var result []string
			result = append(result, fmt.Sprintf("\n📈 %s", exchange.GetName()))
			result = append(result, exchangeRates...)
			maxRates := 20
			if len(exchangeRates) > maxRates {
				exchangeRates = exchangeRates[:maxRates]
				result = append(result, fmt.Sprintf("... и еще %d записей", len(exchangeRates)-maxRates))
			}
			result = append(result, "────────────────────────────")
			text := strings.Join(result, "\n")
			msg := tgbotapi.NewMessage(chatID, text)
			msg.ParseMode = "MarkdownV2"
			if _, err := b.bot.Send(msg); err != nil {
				log.Printf("Ошибка отправки сообщения: %v", err)
			}
		} else {
			thresholdPercent := threshold * 100
			noRatesMsg := fmt.Sprintf("_На бирже %s нет ставок фандинга, превышающих %.2f%%_", exchange.GetName(), thresholdPercent)
			msg := tgbotapi.NewMessage(chatID, noRatesMsg)
			msg.ParseMode = "MarkdownV2"
			if _, err := b.bot.Send(msg); err != nil {
				log.Printf("Ошибка отправки сообщения: %v", err)
			}
		}
	}
}

func (b *Bot) handleUnsubscribe(msg *tgbotapi.Message) {
	log.Printf("Обработка команды unsubscribe от пользователя %s", msg.From.UserName)
	subscribersLock.Lock()
	delete(subscribers, msg.Chat.ID)
	subscribersLock.Unlock()
	saveSettings()
	b.sendLongMessage(msg.Chat.ID, "Вы успешно отписались от уведомлений.")
}

func (b *Bot) startBroadcastLoop() {
	const interval = 5 * time.Minute
	b.broadcastFundingRates() // первая рассылка сразу
	for {
		next := time.Now().Add(interval)
		for {
			now := time.Now()
			if now.After(next) {
				break
			}
			left := next.Sub(now)
			log.Printf("До следующей рассылки осталось: %v", left.Truncate(time.Second))
			time.Sleep(1 * time.Minute)
		}
		b.broadcastFundingRates()
	}
}

func (b *Bot) broadcastFundingRates() {
	subscribersLock.Lock()
	ids := make([]int64, 0, len(subscribers))
	for id := range subscribers {
		ids = append(ids, id)
	}
	subscribersLock.Unlock()

	if len(ids) == 0 {
		return
	}

	rates := b.cache.GetAllRates()
	if len(rates) == 0 {
		return
	}

	for _, chatID := range ids {
		threshold, _ := getUserThreshold(chatID)
		formattedRates := formatRates(rates, threshold)
		if formattedRates != "" {
			b.sendLongMessage(chatID, formattedRates)
		}
	}
}

// sendLongMessage отправляет длинное сообщение, разбивая его на части, если оно слишком большое
func (b *Bot) sendLongMessage(chatID int64, text string) {
	const maxLength = 4000

	// Если есть <pre>...</pre> блоки, разбиваем только по ним
	if strings.Contains(text, "<pre>") {
		blocks := splitByPreBlocks(text)
		for i, block := range blocks {
			if len(block) <= maxLength {
				msg := tgbotapi.NewMessage(chatID, block)
				msg.ParseMode = "HTML"
				if _, err := b.bot.Send(msg); err != nil {
					log.Printf("Ошибка отправки блока %d/%d: %v", i+1, len(blocks), err)
				}
			} else {
				// Если блок всё равно слишком большой, отправляем как есть (Telegram сам обрежет)
				msg := tgbotapi.NewMessage(chatID, block[:maxLength])
				msg.ParseMode = "HTML"
				if _, err := b.bot.Send(msg); err != nil {
					log.Printf("Ошибка отправки длинного блока %d/%d: %v", i+1, len(blocks), err)
				}
			}
		}
		return
	}

	// Обычная логика для текстов без <pre>
	if len(text) <= maxLength {
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = "HTML"
		if _, err := b.bot.Send(msg); err != nil {
			log.Printf("Ошибка отправки сообщения: %v", err)
		}
		return
	}

	// Разбиваем по строкам
	lines := strings.Split(text, "\n")
	currentPart := ""
	parts := []string{}
	for _, line := range lines {
		if len(currentPart)+len(line)+1 > maxLength {
			if currentPart != "" {
				parts = append(parts, currentPart)
				currentPart = ""
			}
			if len(line) > maxLength {
				for i := 0; i < len(line); i += maxLength {
					end := i + maxLength
					if end > len(line) {
						end = len(line)
					}
					parts = append(parts, line[i:end])
				}
			} else {
				currentPart = line
			}
		} else {
			if currentPart != "" {
				currentPart += "\n"
			}
			currentPart += line
		}
	}
	if currentPart != "" {
		parts = append(parts, currentPart)
	}
	for i, part := range parts {
		msg := tgbotapi.NewMessage(chatID, part)
		msg.ParseMode = "HTML"
		if _, err := b.bot.Send(msg); err != nil {
			log.Printf("Ошибка отправки части %d/%d: %v", i+1, len(parts), err)
		}
	}
}

// splitByPreBlocks разбивает текст на блоки по <pre>...</pre>
func splitByPreBlocks(text string) []string {
	var blocks []string
	start := 0
	for {
		preStart := strings.Index(text[start:], "<pre>")
		if preStart == -1 {
			if start < len(text) {
				blocks = append(blocks, text[start:])
			}
			break
		}
		preStart += start
		preEnd := strings.Index(text[preStart:], "</pre>")
		if preEnd == -1 {
			// Нет закрывающего тега, добавляем остаток
			blocks = append(blocks, text[preStart:])
			break
		}
		preEnd += preStart + len("</pre>")
		if preStart > start {
			blocks = append(blocks, text[start:preStart])
		}
		blocks = append(blocks, text[preStart:preEnd])
		start = preEnd
	}
	return blocks
}

func (b *Bot) handleStart(msg *tgbotapi.Message) {
	log.Printf("Обработка команды start от пользователя %s", msg.From.UserName)
	text := "Привет! Я бот для мониторинга ставок фандинга.\n" +
		"Доступные команды:\n" +
		"/rates - показать текущие ставки фандинга\n" +
		"/subscribe - подписаться на уведомления\n" +
		"/unsubscribe - отписаться от уведомлений\n" +
		"/threshold - показать текущий порог\n" +
		"/threshold X.XXX - установить новый порог (например: /threshold 0.1)"

	b.sendLongMessage(msg.Chat.ID, text)
}

func formatRates(rates map[string][]exchanges.FundingRate, threshold float64) string {
	log.Printf("Форматирование ставок с порогом %.6f", threshold)
	var result []string

	for exchangeName, exchangeRates := range rates {
		log.Printf("Обработка %d ставок с биржи %s", len(exchangeRates), exchangeName)

		// Сортируем ставки по модулю
		sort.SliceStable(exchangeRates, func(i, j int) bool {
			return math.Abs(exchangeRates[i].Rate) > math.Abs(exchangeRates[j].Rate)
		})

		var formattedRates []string
		var filteredCount int
		for _, rate := range exchangeRates {
			absRate := math.Abs(rate.Rate)
			if absRate >= threshold {
				log.Printf("[%s] %s: rate=%.6f%%, threshold=%.6f%%, проходит фильтр",
					exchangeName, rate.Symbol, rate.Rate*100, threshold*100)
				paymentTime := rate.NextFunding
				if paymentTime != "Неизвестно" {
					if t, err := time.Parse(time.RFC3339, rate.NextFunding); err == nil {
						if exchangeName == "HTX" {
							paymentTime = t.Format("02.01.2006 15:04")
						} else {
							paymentTime = t.Format("02.01.2006 15:04 MST")
						}
					}
				}
				payDirection := ""
				payEmoji := ""
				if rate.Rate > 0 {
					payDirection = "Long → Short"
					payEmoji = "⬆️"
				} else if rate.Rate < 0 {
					payDirection = "Short → Long"
					payEmoji = "⬇️"
				}
				line := fmt.Sprintf("%-12s %+8.4f%%  %s  %-12s  (выплата: %s)",
					rate.Symbol, rate.Rate*100, payEmoji, payDirection, paymentTime)
				formattedRates = append(formattedRates, line)
			} else {
				filteredCount++
			}
		}

		log.Printf("Биржа %s: отфильтровано %d ставок из %d",
			exchangeName, filteredCount, len(exchangeRates))

		if len(formattedRates) > 0 {
			result = append(result, fmt.Sprintf("\n<b>📈 %s</b>", exchangeName))
			if len(formattedRates) > 0 {
				result = append(result, "<pre>"+strings.Join(formattedRates, "\n")+"</pre>")
			}
			maxRates := 20
			if len(formattedRates) > maxRates {
				formattedRates = formattedRates[:maxRates]
				result = append(result, fmt.Sprintf("<i>... и еще %d записей</i>", len(formattedRates)-maxRates))
			}
			result = append(result, "────────────────────────────")
		}
	}

	if len(result) == 0 {
		return "<i>Нет доступных ставок фандинга, превышающих порог</i>"
	}

	return strings.Join(result, "\n")
}

// handleThreshold обрабатывает команду установки порога
func (b *Bot) handleThreshold(msg *tgbotapi.Message) {
	log.Printf("Обработка команды threshold от пользователя %s", msg.From.UserName)

	// Получаем аргумент команды
	args := strings.TrimSpace(msg.CommandArguments())
	if args == "" {
		// Если аргумент не указан, показываем текущий порог
		threshold, source := getUserThreshold(msg.Chat.ID)
		response := fmt.Sprintf("Текущий порог: %.3f%% (источник: %s)\nДля установки нового порога используйте команду /threshold X.XXX",
			threshold*100, source)
		b.sendLongMessage(msg.Chat.ID, response)
		return
	}

	// Убираем знак процента, если он есть
	args = strings.TrimSuffix(args, "%")

	// Парсим новое значение
	threshold, err := strconv.ParseFloat(args, 64)
	if err != nil {
		b.sendLongMessage(msg.Chat.ID, "Ошибка: укажите корректное число, например: /threshold 0.1")
		return
	}

	// Проверяем диапазон
	if threshold <= 0 {
		b.sendLongMessage(msg.Chat.ID, "Ошибка: порог должен быть положительным числом")
		return
	}

	// Если значение указано в процентах (больше 1), конвертируем в десятичную дробь
	if threshold > 1 {
		threshold = threshold / 100
	}

	// Устанавливаем новый порог
	setUserThreshold(msg.Chat.ID, threshold)

	response := fmt.Sprintf("Установлен новый порог: %.3f%%", threshold*100)
	b.sendLongMessage(msg.Chat.ID, response)

	// Сразу показываем ставки с новым порогом
	go b.handleRates(msg)
}
