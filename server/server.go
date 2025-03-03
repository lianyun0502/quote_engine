package server

import (
	"context"
	"fmt"
	"io"
	"net"
	// "time"

	"github.com/lianyun0502/quote_engine"
	quote_proto "github.com/lianyun0502/quote_engine/proto"
	"google.golang.org/grpc"
)

type Strategy struct {
	ID string
	Name string
	Subscribe map[string]string
}

type QuoteSever struct {
	engine quote_engine.IQuoteEngine
	strategy map[string]*Strategy
}

func NewQuoteServer(engine quote_engine.IQuoteEngine, host, port string) (*QuoteSever, error) {
	fmt.Printf("starting gRPC server... host: %s, port: %s\n", host, port)
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%s", host, port))
	if err != nil {
		return nil, fmt.Errorf("failed to listen: %v", err)
	}
	server := &QuoteSever{
		engine: engine,
		strategy: make(map[string]*Strategy),
	}
	qrpcServer := grpc.NewServer()
	quote_proto.RegisterQuoteServiceServer(qrpcServer, server)
	fmt.Println("gRPC server registered")
	go qrpcServer.Serve(lis)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to serve: %v", err)
	// }
	fmt.Println("gRPC server started")
	return server, nil
}

func (q *QuoteSever) ListQuotes(ctx context.Context, res *quote_proto.ListQuotesRequest) (*quote_proto.ListQuotesResponse, error) {
	quotes := q.engine.GetSubscriptions()
	return &quote_proto.ListQuotesResponse{Symbol: quotes}, nil
}

func (q *QuoteSever) SubscribeCoin(stream quote_proto.QuoteService_SubscribeCoinServer) error {
	var subs []string
	for {
		subInfo, err := stream.Recv()
		if err != nil && err == io.EOF {
			if err == io.EOF {
				break
			}
			return err
		} 
		fmt.Println(subInfo)
		strgy, ok := q.strategy[subInfo.GetID()]
		if !ok {
			return fmt.Errorf("strategy %s not registered", subInfo.Symbol)
		}
		if _, ok := strgy.Subscribe[subInfo.Symbol]; ok {
			return fmt.Errorf("strategy %s already subscribed to %s", subInfo.GetID(), subInfo.Symbol)
		}
		strgy.Subscribe[subInfo.Symbol] = subInfo.Symbol
		subs = append(subs, subInfo.Symbol)
	}
	q.engine.Subscribe(subs)
	return nil
}

func (q *QuoteSever) UnsubscribeCoin(stream quote_proto.QuoteService_UnsubscribeCoinServer) error {
	var subs []string
	for {
		subInfo, err := stream.Recv()
		if err != nil && err == io.EOF{
			break
		}
		fmt.Println(subInfo)
		strgy, ok := q.strategy[subInfo.GetID()]
		if !ok {
			return fmt.Errorf("strategy %s not registered", strgy.Name)
		}
		if _, ok := strgy.Subscribe[subInfo.Symbol]; !ok {
			return fmt.Errorf("strategy %s not subscribed to %s", strgy.Name, subInfo.Symbol)
		}
		for k := range strgy.Subscribe {
			if k == subInfo.ID {
				continue
			}
			if _, ok := strgy.Subscribe[subInfo.ID]; ok {
				return fmt.Errorf("another strategy %s has subscribed to %s", strgy.Name, subInfo.Symbol)
			}
		}
		delete(strgy.Subscribe, subInfo.Symbol)
		subs = append(subs, subInfo.Symbol)
	}
	q.engine.Unsubscribe(subs)
	return nil
}

func (q *QuoteSever) RegisterStrategy(ctx context.Context, req *quote_proto.RegisterStrategyRequest) (*quote_proto.RegisterStrategyResponse, error) {
	if _, ok := q.strategy[req.ID]; ok {
		return nil, fmt.Errorf("strategy %s already exists", req.ID)
	}else{
		q.strategy[req.ID] = &Strategy{
			ID: req.ID,
			Name: req.Name,
			Subscribe: make(map[string]string),
		}
		fmt.Println(q.strategy[req.ID])
	}
	return &quote_proto.RegisterStrategyResponse{Result: 0}, nil
}

func (q *QuoteSever) UnregisterStrategy(ctx context.Context, req *quote_proto.RegisterStrategyRequest) (*quote_proto.RegisterStrategyResponse, error) {
	if _, ok := q.strategy[req.ID]; !ok {
		return nil, fmt.Errorf("strategy %s not exists", req.ID)
	}
	delete(q.strategy, req.ID)
	return &quote_proto.RegisterStrategyResponse{Result: 0}, nil
}

func (q *QuoteSever) ListRegisteredStrategies(ctx context.Context, req *quote_proto.ListRegisteredStrategiesRequest) (*quote_proto.ListRegisteredStrategiesResponse, error) {
	strategies := make([]*quote_proto.Strategy, 0)
	for id, subscribes := range q.strategy {
		strategy := &quote_proto.Strategy{
			ID: id,
			Name: subscribes.Name,
			Subscribe: make([]string, 0),
		}
		for _, subscribe := range subscribes.Subscribe {
			strategy.Subscribe = append(strategy.Subscribe, subscribe)
		}
		strategies = append(strategies, strategy)
	}
	return &quote_proto.ListRegisteredStrategiesResponse{Strategy: strategies}, nil
}