// Package config читает настройки сервиса из переменных окружения.
package config

import (
	"os"
	"time"
)

type Config struct {
	Addr            string        // адрес HTTP-сервера
	DBPath          string        // путь к файлу SQLite
	StreamBaseURL   string        // база для ссылок на HLS-потоки
	ShutdownTimeout time.Duration // сколько ждём завершения запросов по SIGTERM
}

func Load() Config {
	return Config{
		Addr:            env("IPTV_ADDR", ":8080"),
		DBPath:          env("IPTV_DB_PATH", "iptv.db"),
		StreamBaseURL:   env("IPTV_STREAM_BASE_URL", "http://stream.example.com"),
		ShutdownTimeout: 10 * time.Second,
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
