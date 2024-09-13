package main

import (
	"context"

	. "github.com/lianyun0502/quote_engine"
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
	InitLogger(logger, &config.Log)
	ctx, cancel := context.WithCancel(context.Background())
	datastorage.NewDataStorage(ctx, config, logger)
	quoteEngine := NewQuoteEngine(config, logger)

	WaitForClose(quoteEngine.Logger, ctx)
	cancel()
}

