package config

import "os"

type Config struct {
	Host             string // bind address, e.g. "127.0.0.1" or "" for all interfaces
	Port             string
	BinanceAPIKey    string
	BinanceSecretKey string
	// UID used to authenticate incoming webhook signals
	AuthUID string
	// MongoDB connection URI; empty means persistence is disabled
	MongoURI string
}

func Load() *Config {
	return &Config{
		Host:             env("HOST", "127.0.0.1"),
		Port:             env("PORT", "8080"),
		BinanceAPIKey:    env("BINANCE_API_KEY", ""),
		BinanceSecretKey: env("BINANCE_SECRET_KEY", ""),
		AuthUID:          env("AUTH_UID", ""),
		MongoURI:         env("MONGODB_URI", ""),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
