package datastorage

import (
	"context"
	"github.com/lianyun0502/quote_engine/configs"
	"github.com/lianyun0502/shm"
	"github.com/sirupsen/logrus"
)

type DataStorage struct {
	sub_map    map[string]map[string]*shm.Subscriber
	DoneSignal context.Context
	doneFunc   func()
}

func NewDataStorage(ctx context.Context, cfg *configs.Config, logger *logrus.Logger) *DataStorage {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	obj := &DataStorage{
		sub_map:    make(map[string]map[string]*shm.Subscriber),
		DoneSignal: ctx,
		doneFunc:   cancel,
	}
	for _, wsCfg := range cfg.Websocket {
		sub_map := NewSubscriberMap(ctx, wsCfg.Publisher, logger)
		obj.sub_map[wsCfg.Exchange] = sub_map
		for topic, sub := range sub_map {
			switch topic {
			case "orderbook":
				sub.Handle = WithOrderbookTxtHandle(ctx, logger)
			case "tickers":
				sub.Handle = WithTickerTxtHandle(ctx, logger)
			}
			go sub.ReadLoop()

		}
	}
	return obj
}

func NewDataStorage2(ctx context.Context, cfg *configs.Config, logger *logrus.Logger) *DataStorage {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	obj := &DataStorage{
		sub_map:    make(map[string]map[string]*shm.Subscriber),
		DoneSignal: ctx,
		doneFunc:   cancel,
	}
	for _, wsCfg := range cfg.Websocket {
		sub_map := NewSubscriberMap(ctx, wsCfg.Publisher, logger)
		obj.sub_map[wsCfg.Exchange] = sub_map
		for topic, sub := range sub_map {
			switch topic {
			case "orderbook":
				sub.Handle = WithOrderbookTxtHandle(ctx, logger)
			case "Trade":
				sub.Handle = WithTradeTxtHandle(ctx, logger)
			case "ticker":
				sub.Handle = WithTickerTxtHandle(ctx, logger)
			}
			go sub.ReadLoop()

		}
	}
	return obj
}

func NewSubscriberMap(ctx context.Context, configs []configs.PublisherConfig, logger *logrus.Logger) map[string]*shm.Subscriber {
	sub_map := make(map[string]*shm.Subscriber)
	for _, pub := range configs {
		if !pub.Store {
			continue
		}
		sub_map[pub.Topic] = shm.NewSubscriber(pub.Skey, pub.Size)
	}
	return sub_map
}

func (ds *DataStorage) Close() {
	for _, sub_map := range ds.sub_map {
		for _, sub := range sub_map {
			sub.Close()
		}
	}
	ds.doneFunc()
}
