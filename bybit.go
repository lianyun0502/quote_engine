package quote_engine

import (
	"encoding/json"
	"regexp"
	"time"

	"github.com/lianyun0502/exchange_conn/v1/bybit_conn/data_stream"
	"github.com/lianyun0502/shm"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fastjson"
)

// 將 bybit 的 symbol的綴詞 轉換成 topic，如果不符合任何規則，則回傳原本的 symbol
func ByBitSymbolToTopic(symbol string) string {
	switch {
	case regexp.MustCompile("publicTrade").MatchString(symbol):
		return "publicTrade"
	case regexp.MustCompile("orderbook").MatchString(symbol):
		return "orderbook"
	case regexp.MustCompile("tickers").MatchString(symbol):
		return "tickers"
	default:
		return symbol
	}
}


func WithTradeHandler(logger *logrus.Logger, pub *shm.Publisher) func([]byte) {
	logger.Debug("trade handler")
	parser := data_stream.NewTrade()	
	return func(rawData []byte) {
		data, err := parser.Update(rawData)
		if err != nil {
			logger.Debug(string(rawData))
			logger.Error(err)
			return
		}
		if data != nil {
			for _, data := range data.Trades {
				jsonData, err := json.Marshal(data)
				if err != nil {
					logger.Error(err)
					return
				}
				pub.Write(jsonData)
			}
		}
	}
}

func WithOrderBookHandler(logger *logrus.Logger, pub *shm.Publisher) func([]byte) {
	logger.Debug("orderbook handler")
	parser := data_stream.NewOrderBook()
	return func(rawData []byte) {
		data, err := parser.Update(rawData)
		if err != nil {
			logger.Debug(string(rawData))
			logger.Error(err)
			return
		}
		if data != nil {
			jsonData, err := json.Marshal(data)
			if err != nil {
				logger.Error(err)
				return
			}
			pub.Write(jsonData)
		}
	}
}

func WithTickerHandler(logger *logrus.Logger, pub *shm.Publisher) func([]byte) {
	logger.Debug("ticker handler")
	parser := data_stream.NewMarketData()
	return func(rawData []byte) {
		data, err := parser.Update(rawData)
		if err != nil {
			logger.Debug(string(rawData))
			logger.Error(err)
			return
		}
		if data != nil {
			jsonData, err := json.Marshal(data)
			if err != nil {
				logger.Error(err)
				return
			}
			pub.Write(jsonData)
		}
	}
}


// 給定 WsClientConfig 和 logger 生成一個 bybit 的 message handler 的 closure, 
func WithBybitMessageHandler(wsCfg *WsClientConfig, logger *logrus.Logger) func([]byte) {
	// map of publisher, key is topic, value is publisher
	handleMap := make(map[string]func([]byte))
	for _, pub := range wsCfg.Publisher {
		topic := ByBitSymbolToTopic(pub.Topic)
		switch topic{
		case "publicTrade":
			handleMap[topic] = WithTradeHandler(logger, shm.NewPublisher(pub.Skey, pub.Size))
		case "orderbook":
			handleMap[topic] = WithOrderBookHandler(logger, shm.NewPublisher(pub.Skey, pub.Size))
		case "tickers":
			handleMap[topic] = WithTickerHandler(logger, shm.NewPublisher(pub.Skey, pub.Size))
		default:
			logger.Errorf("can not gen handler for unknown topic: %s", topic)
		}
	}
	return func(rawData []byte) {
		logger.Debugf("rev time: %d", time.Now().UnixNano() / int64(time.Millisecond))
		v := fastjson.MustParseBytes(rawData)
		symbol := string(v.GetStringBytes("topic"))
		topic := ByBitSymbolToTopic(symbol)
		if handler, ok := handleMap[topic]; ok {
			logger.Debugf("rev topic: %s", topic)
			handler(rawData)
		}else{
			logger.Errorf("unknown topic: %s", topic)
		}
	}
}