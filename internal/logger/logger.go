package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

type ExchangeLogger struct {
	name   string
	logger *log.Logger
	file   *os.File
	mu     sync.Mutex
}

var loggers = make(map[string]*ExchangeLogger)
var loggersMu sync.RWMutex

// GetExchangeLogger возвращает логгер для конкретной биржи
func GetExchangeLogger(exchangeName string) *ExchangeLogger {
	loggersMu.RLock()
	if logger, exists := loggers[exchangeName]; exists {
		loggersMu.RUnlock()
		return logger
	}
	loggersMu.RUnlock()

	loggersMu.Lock()
	defer loggersMu.Unlock()

	// Проверяем еще раз после получения блокировки записи
	if logger, exists := loggers[exchangeName]; exists {
		return logger
	}

	// Создаем новый логгер
	logger, err := newExchangeLogger(exchangeName)
	if err != nil {
		log.Printf("Ошибка создания логгера для %s: %v", exchangeName, err)
		return nil
	}

	loggers[exchangeName] = logger
	return logger
}

// newExchangeLogger создает новый логгер для биржи
func newExchangeLogger(exchangeName string) (*ExchangeLogger, error) {
	// Создаем директорию logs если её нет
	logsDir := "logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return nil, fmt.Errorf("не удалось создать директорию logs: %v", err)
	}

	// Создаем файл для биржи
	filename := filepath.Join(logsDir, fmt.Sprintf("%s.log", exchangeName))
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("не удалось создать файл лога %s: %v", filename, err)
	}

	// Создаем логгер
	logger := log.New(file, "", log.LstdFlags|log.Lshortfile)

	return &ExchangeLogger{
		name:   exchangeName,
		logger: logger,
		file:   file,
	}, nil
}

// Printf логирует сообщение в файл биржи
func (el *ExchangeLogger) Printf(format string, v ...interface{}) {
	if el == nil || el.logger == nil {
		log.Printf("[%s] %s", el.name, fmt.Sprintf(format, v...))
		return
	}

	el.mu.Lock()
	defer el.mu.Unlock()

	el.logger.Printf(format, v...)

	// Также выводим в общий лог с префиксом биржи
	log.Printf("[%s] %s", el.name, fmt.Sprintf(format, v...))
}

// Println логирует сообщение в файл биржи
func (el *ExchangeLogger) Println(v ...interface{}) {
	if el == nil || el.logger == nil {
		log.Printf("[%s] %s", el.name, fmt.Sprint(v...))
		return
	}

	el.mu.Lock()
	defer el.mu.Unlock()

	el.logger.Println(v...)

	// Также выводим в общий лог с префиксом биржи
	log.Printf("[%s] %s", el.name, fmt.Sprint(v...))
}

// Close закрывает файл лога
func (el *ExchangeLogger) Close() error {
	if el == nil || el.file == nil {
		return nil
	}

	el.mu.Lock()
	defer el.mu.Unlock()

	return el.file.Close()
}

// CloseAll закрывает все логгеры
func CloseAll() {
	loggersMu.Lock()
	defer loggersMu.Unlock()

	for _, logger := range loggers {
		logger.Close()
	}
}
