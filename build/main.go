package main

import (
	"context"
	// "time"

	"github.com/lianyun0502/quote_engine"
	"github.com/lianyun0502/quote_engine/configs"
	"github.com/lianyun0502/quote_engine/data_storage"
	"github.com/lianyun0502/quote_engine/server"
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
	engine := quote_engine.NewBybitQuoteEngine(&config.Websocket[0], logger)
	engine.SetSubscribeInstruments()
	server.NewQuoteServer(engine, config.GRPCServer.Host, config.GRPCServer.Port)
	// engine.AddScheduler("UpdateSubscribeInstruments", 5*time.Minute, engine.UpdateSubscribeMap)
	quote_engine.WaitForClose(logger, ctx)
	cancel()
}

