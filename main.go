package quote_engine

import (

	"github.com/sirupsen/logrus"

)


func main () {
	config, err := LoadConfig("config.yaml")
	if err != nil {
		logrus.Println(err)
		return
	} 
	
	quoteEngine := NewQuoteEngine(config)

	quoteEngine.WsAgent.StartLoop()

	<- quoteEngine.DoneSignal
}