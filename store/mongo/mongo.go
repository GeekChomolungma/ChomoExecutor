package mongo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/GeekChomolungma/ChomoExecutor/store"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	defaultDatabase = "chomo"
	pingTimeout     = 10 * time.Second
)

type mongoStore struct {
	db *mongo.Database
}

// New connects to MongoDB and returns an OrderStore backed by the given URI.
func New(ctx context.Context, uri string) (store.OrderStore, error) {
	opts := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, fmt.Errorf("mongo store: connect: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	if err := client.Ping(pingCtx, nil); err != nil {
		return nil, fmt.Errorf("mongo store: ping: %w", err)
	}
	return &mongoStore{db: client.Database(defaultDatabase)}, nil
}

func (s *mongoStore) SaveOrderResult(ctx context.Context, record *store.OrderRecord) error {
	col := s.db.Collection(orderCollection(record.MarketType))
	if _, err := col.InsertOne(ctx, record); err != nil {
		return fmt.Errorf("mongo store: insert order result: %w", err)
	}
	return nil
}

// orderCollection maps a market type to its dedicated collection name.
// Adding a new market type only requires extending this function.
func orderCollection(marketType string) string {
	switch strings.ToLower(marketType) {
	case "spot":
		return "order_results_spot"
	case "futures":
		return "order_results_futures"
	default:
		return "order_results_unknown"
	}
}
