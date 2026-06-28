//go:build integration

package mongo_test

import (
	"context"
	"testing"
	"time"

	tcmongo "github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/GeekChomolungma/ChomoExecutor/store"
	mongostore "github.com/GeekChomolungma/ChomoExecutor/store/mongo"
)

// startMongo spins up a real MongoDB container and returns (uri, cleanup).
func startMongo(t *testing.T) (string, func()) {
	t.Helper()
	ctx := context.Background()
	container, err := tcmongo.Run(ctx, "mongo:7")
	if err != nil {
		t.Fatalf("start mongo container: %v", err)
	}
	uri, err := container.ConnectionString(ctx)
	if err != nil {
		container.Terminate(ctx)
		t.Fatalf("get connection string: %v", err)
	}
	return uri, func() { container.Terminate(ctx) }
}

// rawClient returns a direct mongo client for reading documents in assertions.
func rawClient(t *testing.T, uri string) *mongo.Client {
	t.Helper()
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatalf("raw client connect: %v", err)
	}
	return client
}

func baseRecord(marketType string) *store.OrderRecord {
	return &store.OrderRecord{
		SignalID:   "sig-test",
		UID:        "uid-test",
		Exchange:   "binance",
		MarketType: marketType,
		Symbol:     "BTCUSDT",
		Side:       "BUY",
		OrderType:  "MARKET",
		Quantity:   0.01,
		CreatedAt:  time.Now().UTC(),
		OrderID:    "order-001",
		Status:     "FILLED",
		FilledQty:  0.01,
		AvgPrice:   50000.0,
	}
}

// TestSaveOrderResult_CollectionRouting verifies that spot and futures records
// land in their own dedicated collections.
func TestSaveOrderResult_CollectionRouting(t *testing.T) {
	uri, cleanup := startMongo(t)
	defer cleanup()

	ctx := context.Background()
	s, err := mongostore.New(ctx, uri)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	cases := []struct {
		marketType string
		wantCol    string
	}{
		{"spot", "order_results_spot"},
		{"futures", "order_results_futures"},
		{"unknown_type", "order_results_unknown"},
	}

	client := rawClient(t, uri)
	db := client.Database("chomo")

	for _, tc := range cases {
		t.Run(tc.marketType, func(t *testing.T) {
			rec := baseRecord(tc.marketType)
			if err := s.SaveOrderResult(ctx, rec); err != nil {
				t.Fatalf("SaveOrderResult: %v", err)
			}

			count, err := db.Collection(tc.wantCol).CountDocuments(ctx, bson.M{"signal_id": "sig-test"})
			if err != nil {
				t.Fatalf("count documents: %v", err)
			}
			if count != 1 {
				t.Errorf("collection %q: got %d documents, want 1", tc.wantCol, count)
			}
		})
	}
}

// TestSaveOrderResult_DocumentContent verifies that persisted fields match the record.
func TestSaveOrderResult_DocumentContent(t *testing.T) {
	uri, cleanup := startMongo(t)
	defer cleanup()

	ctx := context.Background()
	s, err := mongostore.New(ctx, uri)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	rec := baseRecord("futures")
	rec.SignalID = "content-check"
	if err := s.SaveOrderResult(ctx, rec); err != nil {
		t.Fatalf("SaveOrderResult: %v", err)
	}

	client := rawClient(t, uri)
	var doc bson.M
	err = client.Database("chomo").
		Collection("order_results_futures").
		FindOne(ctx, bson.M{"signal_id": "content-check"}).
		Decode(&doc)
	if err != nil {
		t.Fatalf("find document: %v", err)
	}

	checks := map[string]interface{}{
		"uid":        "uid-test",
		"symbol":     "BTCUSDT",
		"side":       "BUY",
		"order_type": "MARKET",
		"order_id":   "order-001",
		"status":     "FILLED",
	}
	for field, want := range checks {
		if got := doc[field]; got != want {
			t.Errorf("field %q: got %v, want %v", field, got, want)
		}
	}
}
