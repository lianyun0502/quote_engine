package main

import (
	"context"

	"github.com/lianyun0502/quote_engine"
	"github.com/lianyun0502/quote_engine/configs"
	"github.com/lianyun0502/quote_engine/data_storage"
	"github.com/sirupsen/logrus"
)

func main() {
	config, err := configs.LoadConfig("config.yaml")
	if err != nil {
		logrus.Println(err)
		return
	}
	logger := logrus.New()
	quote_engine.InitLogger(logger, &config.Log)
	ctx, cancel := context.WithCancel(context.Background())
	if config.Data.Save {
		datastorage.NewDataStorage(ctx, config, logger)
	}
	config.Websocket[0].WsPoolSize = 1
	engine := quote_engine.NewBybitQuoteEngine(&config.Websocket[0], logger)
	engine.SetSubscribeInstruments(&config.Websocket[0])

	quote_engine.WaitForClose(logger, ctx)
	cancel()
}

