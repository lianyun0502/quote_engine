package server

import (
	"fmt"
	"io"
	"net"

	"github.com/lianyun0502/quote_engine"
	quote_proto "github.com/lianyun0502/quote_engine/proto"
	"google.golang.org/grpc"
)

type QuoteSever struct {
	engine quote_engine.IQuoteEngine
}

func NewQuoteServer(engine quote_engine.IQuoteEngine, host, port string) (*QuoteSever, error) {
	fmt.Printf("starting gRPC server... host: %s, port: %s\n", host, port)
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%s", host, port))
	if err != nil {
		return nil, fmt.Errorf("failed to listen: %v", err)
	}
	server := &QuoteSever{
		engine: engine,
	}
	qrpcServer := grpc.NewServer()
	quote_proto.RegisterQuoteServiceServer(qrpcServer, server)
	if err := qrpcServer.Serve(lis); err != nil {
		return nil, fmt.Errorf("failed to serve: %v", err)
	}
	return server, nil
}

func (q *QuoteSever) ListQuotes(res *quote_proto.ListQuotesRequest, stream quote_proto.QuoteService_ListQuotesServer) error {
	quotes := q.engine.GetSubscriptions()
	for _, quote := range quotes {
		stream.Send(&quote_proto.Subscribe{
			Symbol: quote,
		})
	}
	return nil
}

func (q *QuoteSever) SubscribeCoin(stream quote_proto.QuoteService_SubscribeCoinServer) error {
	var subs []string
	for {
		sub, err := stream.Recv()
		if err != nil && err == io.EOF {
			if err == io.EOF {
				break
			}
			return err
		} 
		fmt.Println(sub)
		subs = append(subs, sub.Symbol)
	}
	q.engine.SubscribeQuotes(subs)
	return nil
}

func (q *QuoteSever) UnsubscribeCoin(stream quote_proto.QuoteService_UnsubscribeCoinServer) error {
	var subs []string
	for {
		sub, err := stream.Recv()
		if err != nil && err == io.EOF{
			break
		}
		fmt.Println(sub)
		subs = append(subs, sub.Symbol)
	}
	q.engine.UnsubscribeQuotes(subs)
	return nil
}