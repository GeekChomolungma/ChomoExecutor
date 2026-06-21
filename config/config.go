package config

import "os"

type Config struct {
	Port             string
	BinanceAPIKey    string
	BinanceSecretKey string
	// Switch between production (https://fapi.binance.com) and testnet
	BinanceBaseURL string
	// UID used to authenticate incoming webhook signals
	AuthUID string
}

func Load() *Config {
	return &Config{
		Port:             env("PORT", "8080"),
		BinanceAPIKey:    env("BINANCE_API_KEY", ""),
		BinanceSecretKey: env("BINANCE_SECRET_KEY", ""),
		BinanceBaseURL:   env("BINANCE_BASE_URL", "https://fapi.binance.com"),
		AuthUID:          env("AUTH_UID", ""),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
