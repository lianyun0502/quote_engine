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


// 給定 WsClientConfig 和 logger 生成一個 bybit 的 message handler 的 closure, 
func WithBybitMessageHandler(wsCfg *WsClientConfig, logger *logrus.Logger) func([]byte) {
	shm.Logger = logger

	parser := data_stream.NewDataParser()

	// map of publisher, key is topic, value is publisher
	pubMap := make(map[string]*shm.Publisher)
	for _, pub := range wsCfg.Publisher {
		topic := ByBitSymbolToTopic(pub.Topic)
		pubMap[topic] = shm.NewPublisher(pub.Skey, pub.Size)
	}
	
	return func(rawData []byte) {
		logger.Debugf("rev time: %d", time.Now().UnixNano() / int64(time.Millisecond))
		v := fastjson.MustParseBytes(rawData)
		symbol := string(v.GetStringBytes("topic"))
		topic := ByBitSymbolToTopic(symbol)
		data, err := parser.Parse(rawData)
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
			pubMap[topic].Write(jsonData)
		}
	
	}
}