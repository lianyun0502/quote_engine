package client

import (
	"fmt"
	// "io"
	"time"
	"context"

	// "github.com/lianyun0502/quote_engine"
	quote_proto "github.com/lianyun0502/quote_engine/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type QuoteClient struct {
	client quote_proto.QuoteServiceClient
}

func NewQuoteClient(host, port string) (*QuoteClient, error) {
	fmt.Println("starting gRPC client...")
	conn, err := grpc.NewClient(fmt.Sprintf("%s:%s", host, port), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %v", err)
	}
	// defer conn.Close()
	client := quote_proto.NewQuoteServiceClient(conn)
	return &QuoteClient{client: client}, nil
}

func (q *QuoteClient) ListQuotes() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := q.client.ListQuotes(ctx, &quote_proto.ListQuotesRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list quotes: %v", err)
	}
	return resp.Symbol, nil
}

func (q *QuoteClient) SubscribeCoin(quotes []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stream, err := q.client.SubscribeCoin(ctx)
	if err != nil {
		return fmt.Errorf("failed to subscribe coin: %v", err)
	}
	for _, quote := range quotes {
		if err := stream.Send(&quote_proto.Subscribe{Symbol: quote}); err != nil {
			return fmt.Errorf("failed to send quote: %v", err)
		}
	}
	_, err = stream.CloseAndRecv()
	if err != nil {
		return fmt.Errorf("failed to close and receive: %v", err)
	}
	return nil
}

func (q *QuoteClient) Close() {
}

func (q *QuoteClient) UnsubscribeCoin(quotes []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stream, err := q.client.UnsubscribeCoin(ctx)
	if err != nil {
		return fmt.Errorf("failed to unsubscribe coin: %v", err)
	}
	for _, quote := range quotes {
		if err := stream.Send(&quote_proto.Unsubscribe{Symbol: quote}); err != nil {
			return fmt.Errorf("failed to send quote: %v", err)
		}
	}
	_, err = stream.CloseAndRecv()
	if err != nil {
		return fmt.Errorf("failed to close and receive: %v", err)
	}
	return nil

}


func (q *QuoteClient) RegisterStrategy(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := q.client.RegisterStrategy(ctx, &quote_proto.RegisterStrategyRequest{ID: id})
	if err != nil {
		return fmt.Errorf("failed to register strategy: %v", err)
	}
	return nil
}

func (q *QuoteClient) UnregisterStrategy(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := q.client.UnregisterStrategy(ctx, &quote_proto.RegisterStrategyRequest{ID: id})
	if err != nil {
		return fmt.Errorf("failed to unregister strategy: %v", err)
	}
	return nil
}

func (q *QuoteClient) ListRegisteredStrategies() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := q.client.ListRegisteredStrategies(ctx, &quote_proto.ListRegisteredStrategiesRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list registered strategies: %v", err)
	}
	var strategies []string
	for _, strategy := range resp.Strategy{
		strategies = append(strategies, strategy.GetID())
	}
	return strategies, nil
}