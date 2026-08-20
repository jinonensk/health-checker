package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port      int
	LogLevel  string
	LogFormat string // "json" или "text"
}

func Load() Config {
	// Загружаем .env, если файл есть
	_ = godotenv.Load() // игнорируем ошибку, если файла нет

	port := 8080
	if p, err := strconv.Atoi(os.Getenv("PORT")); err == nil && p > 0 {
		port = p
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	logFormat := os.Getenv("LOG_FORMAT")
	if logFormat == "" {
		logFormat = "text"
	}

	return Config{
		Port:      port,
		LogLevel:  logLevel,
		LogFormat: logFormat,
	}
}
