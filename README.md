# Funding Screener Bot

Telegram-бот для мониторинга ставок фандинга на крупнейших криптовалютных биржах. Позволяет отслеживать высокие ставки фандинга, получать уведомления и легко расширяется за счёт модульной архитектуры.

---

## Возможности
- Получение актуальных ставок фандинга по основным биржам
- Поддержка команд Telegram: /start, /rates, /subscribe, /unsubscribe
- Уведомления о высоких ставках фандинга
- Гибкая модульная архитектура: легко добавить новую биржу через интерфейс
- Корректная работа с длинными сообщениями (разделение на части)
- Поддержка таймзоны для отображения времени фандинга

---

## Архитектура проекта

```
Funding_screener/
├── main.go                # Точка входа, инициализация бота и бирж
├── internal/
│   ├── bot/               # Модуль Telegram-бота
│   │   └── bot.go
│   └── exchanges/         # Модули для каждой биржи и интерфейс Exchange
│       ├── binance.go
│       ├── bybit.go
│       ├── htx.go
│       ├── okx.go
│       ├── gate.go
│       ├── kucoin.go
│       ├── bingx.go
│       └── exchange.go    # Интерфейс Exchange и тип FundingRate
├── subscribers.json       # Список подписчиков Telegram-бота
├── README.md
├── go.mod, go.sum         # Зависимости Go
└── .env.example           # Пример файла переменных окружения
```

---

## Модульная система и расширяемость

Каждая биржа реализует интерфейс `Exchange`:
```go
// internal/exchanges/exchange.go
 type Exchange interface {
     GetName() string
     GetFundingRates() ([]FundingRate, error)
 }
```
Чтобы добавить новую биржу:
1. Создайте файл в internal/exchanges/ (например, myexchange.go)
2. Реализуйте методы интерфейса Exchange
3. Добавьте инициализацию в main.go

---

## Поддерживаемые биржи

- **Binance** (публичный API)
- **Bybit** (публичный API)
- **HTX** (публичный API)
- **OKX** (требуются API-ключи)
- **Gate.io** (публичный API)
- **KuCoin** (публичный API)
- **BingX** (требуются API-ключи)

**Особенности:**
- Для OKX и BingX обязательны API-ключи (см. раздел "Переменные окружения").
- Для остальных бирж используются публичные эндпоинты.
- Для BingX: если API возвращает пустой массив, бот не подставляет тестовые данные, а сообщает об ошибке.
- Для OKX: ставки фандинга могут отсутствовать для части инструментов (особенность API).

---

## Переменные окружения

Создайте файл `.env` на основе `.env.example` и укажите необходимые значения:

```
# Telegram
TELEGRAM_BOT_TOKEN=ваш_telegram_token

# OKX API Keys (обязательны для работы OKX)
OKX_API_KEY=your_okx_api_key
OKX_SECRET_KEY=your_okx_secret_key
OKX_PASSPHRASE=your_okx_passphrase

# BingX API Keys (обязательны для работы BingX)
BINGX_API_KEY=your_bingx_api_key
BINGX_SECRET_KEY=your_bingx_secret_key

# Таймзона (например, Europe/Kyiv)
TIMEZONE=Europe/Kyiv
```

**Важно:**
- Если переменные для OKX или BingX не заданы, соответствующие модули не будут работать корректно.
- Для остальных бирж ключи не требуются.

---

## Установка и запуск

1. Клонируйте репозиторий:
   ```bash
   git clone ...
   cd Funding_screener
   ```
2. Установите зависимости:
   ```bash
   go mod download
   ```
3. Создайте файл `.env` на основе `.env.example` и заполните все необходимые переменные.
4. Запустите бота:
   ```bash
   go run main.go
   ```

---

## Использование

- `/start` — Информация о боте и доступных командах
- `/rates` — Показать текущие высокие ставки фандинга (выше 0.1%)
- `/subscribe` — Подписаться на автоматические уведомления о высоких ставках
- `/unsubscribe` — Отписаться от уведомлений

---

## Логирование и отладка

- Все ключевые действия и ошибки логируются в консоль.
- Для OKX и BingX выводятся подробные логи запросов, заголовков, ответов и ошибок аутентификации.
- Если биржа возвращает ошибку или пустой результат, бот сообщает об этом в Telegram.
- Для длинных сообщений реализовано автоматическое разбиение на части (лимит Telegram — 4096 символов).

---

## Примечания и рекомендации

- Для корректной работы с OKX и BingX обязательно проверьте правильность API-ключей и их наличие в окружении.
- Если BingX возвращает пустой массив data — это ограничение или ошибка на стороне API, бот не подставляет тестовые данные.
- Для добавления новой биржи реализуйте интерфейс Exchange и зарегистрируйте модуль в main.go.
- Таймзона влияет на отображение времени следующего фандинга.


## Зависимости

Для работы проекта необходимы дополнительные пакеты, которые лежат в отдельном репозитории:
- infrastructure: git@github.com:petrixs/cr-infrastructure.git
- transport-bus: git@github.com:petrixs/cr-transport-bus.git

Для автоматической загрузки зависимостей используйте:

```sh
make deps
```

Пакеты будут размещены в папке packages. 