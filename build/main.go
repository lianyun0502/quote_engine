package main

import (
	"context"
	"fmt"
	"time"

	// "time"

	// "time"

	"github.com/lianyun0502/quote_engine"
	"github.com/lianyun0502/quote_engine/configs"
	"github.com/lianyun0502/quote_engine/data_storage"
	// "github.com/lianyun0502/quote_engine/server"
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
		logger.Info("create data storage")
		datastorage.NewDataStorage2(ctx, config, logger)
	}
	engine := quote_engine.NewQuoteEngine(&config.Websocket[0], logger)
	// _ , err = server.NewQuoteServer(engine, config.GRPCServer.Host, config.GRPCServer.Port)
	// if err != nil {
	// 	logrus.Println(err)
	// 	return
	// }
	time.Sleep(20 * time.Second)
	fmt.Println("SetSubscribeInstruments")
	engine.SetSubscribeInstruments()
	quote_engine.WaitForClose(logger, ctx)
	cancel()
}

