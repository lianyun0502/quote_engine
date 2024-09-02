package quote_engine_test

import (
	"testing"
	"github.com/sirupsen/logrus"
	. "github.com/lianyun0502/quote_engine"

)

func TestNewQuoteEngine(t *testing.T) {
	config, err := LoadConfig("config.yaml")
	if err != nil {
		logrus.Println(err)
		return
	} 
	
	quoteEngine := NewQuoteEngine(config)

	quoteEngine.WsAgent.StartLoop()

	<- quoteEngine.DoneSignal
}


