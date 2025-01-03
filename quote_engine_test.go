package quote_engine_test

import (
	"testing"
	"context"

	"github.com/lianyun0502/quote_engine/configs"
	"github.com/lianyun0502/quote_engine"
	"github.com/lianyun0502/quote_engine/server"
	"github.com/sirupsen/logrus"
)


func TestByBitQuoteEngine(t *testing.T){
	cfg, err := configs.LoadConfig("config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	logger := logrus.New()
	quote_engine.InitLogger(logger, &cfg.Log)
	engine := quote_engine.NewBybitQuoteEngine(&cfg.Websocket[0], logger)
	engine.SetSubscribeInstruments()
	server.NewQuoteServer(engine, "localhost", "6666")
	engine.LoadSubscribes()
	quote_engine.WaitForClose(logger, ctx)
}