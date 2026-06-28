package main

import (
	"context"
	"log"

	"github.com/GeekChomolungma/ChomoExecutor/api"
	"github.com/GeekChomolungma/ChomoExecutor/api/handler"
	"github.com/GeekChomolungma/ChomoExecutor/config"
	"github.com/GeekChomolungma/ChomoExecutor/exchange"
	binance "github.com/GeekChomolungma/ChomoExecutor/exchange/binance"
	"github.com/GeekChomolungma/ChomoExecutor/exchange/mock"
	"github.com/GeekChomolungma/ChomoExecutor/service"
	"github.com/GeekChomolungma/ChomoExecutor/store"
	mongostore "github.com/GeekChomolungma/ChomoExecutor/store/mongo"
)

func main() {
	cfg := config.Load()

	var exchanges map[string]exchange.Exchange

	if cfg.BinanceAPIKey == "" {
		log.Println("[main] No BINANCE_API_KEY -- using mock exchange for all venues")
		mockEx := mock.NewExchange(50000.0, 1000.0)
		exchanges = map[string]exchange.Exchange{
			service.RegisterKey("binance", "spot"):    mockEx,
			service.RegisterKey("binance", "futures"): mockEx,
		}
	} else {
		log.Println("[main] Connecting to Binance (spot + futures)")
		exchanges = map[string]exchange.Exchange{
			service.RegisterKey("binance", "spot"):    binance.NewSpotClient(cfg.BinanceAPIKey, cfg.BinanceSecretKey),
			service.RegisterKey("binance", "futures"): binance.NewFuturesClient(cfg.BinanceAPIKey, cfg.BinanceSecretKey),
		}
	}

	// To add a new exchange (e.g. OKX), implement exchange.Exchange and register here:
	// exchanges[service.RegisterKey("okx", "spot")] = okx.NewSpotClient(...)

	var orderStore store.OrderStore
	if cfg.MongoURI != "" {
		var err error
		orderStore, err = mongostore.New(context.Background(), cfg.MongoURI)
		if err != nil {
			log.Fatalf("[main] MongoDB connect failed: %v", err)
		}
		log.Println("[main] MongoDB persistence enabled")
	} else {
		log.Println("[main] No MONGO_URI -- order persistence disabled")
	}

	executor := service.NewExecutor(exchanges, orderStore)
	signalHandler := handler.NewSignalHandler(executor, cfg.AuthUID)
	router := api.NewRouter(signalHandler)

	addr := ":" + cfg.Port
	log.Printf("[main] ChomoExecutor listening on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("[main] server error: %v", err)
	}
}
