package quote_engine

import (
	"encoding/json"
	"regexp"
	"fmt"

	"github.com/lianyun0502/exchange_conn/v1/binance_conn/data_stream"
	"github.com/lianyun0502/shm"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fastjson"
)

func SymbolToTopic(symbol string) string {
	tradeExp := regexp.MustCompile(`trade`)
	aggTradeExp := regexp.MustCompile(`aggTrade`)
	depthExp := regexp.MustCompile(`depth`)
	switch {
	case aggTradeExp.MatchString(symbol):
		return "aggTrade"
	case tradeExp.MatchString(symbol):
		return "trade"
	case depthExp.MatchString(symbol):
		return "depthUpdate"
	default:
		return ""
	}
}

// life time is infinity
func WithBinanceMessageHandler(wsCfg *WsClientConfig, logger *logrus.Logger) func([]byte) {
	var parse func([]byte)(any, error)

	parse = GetBinanceParser

	pubMap := make(map[string]*shm.Publisher)
	for _, pub := range wsCfg.Publisher {
		topic := SymbolToTopic(pub.Topic)
		pubMap[topic] = shm.NewPublisher(pub.Skey, pub.Size)
	}
	return func(rawData []byte) {
		logger.Debugln(string(rawData))
		v := fastjson.MustParseBytes(rawData)
		topic := string(v.GetStringBytes("e"))
		data, err := parse(rawData)
		if err != nil {
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

func GetBinanceParser(rawData []byte) (any, error) {
	v := fastjson.MustParseBytes(rawData)
	topic := string(v.GetStringBytes("e"))
	switch topic{
	case "trade":
		return data_stream.ToNormalTradeData(rawData)
	default:
		return nil, fmt.Errorf("topic %s not found", topic)
	} 
}

// func GetBinancePubMap(rawData []byte) (map[string]*shm.Publisher, error) {
// 	v := fastjson.MustParseBytes(rawData)
// 	topic := string(v.GetStringBytes("e"))
// 	tradeExp := regexp.MustCompile(`Trade`)
// 	switch {
// 	case tradeExp.MatchString(topic):
// 		return data_stream.ToNormalTradeData(rawData)
// 	default:
// 		return nil, fmt.Errorf("topic %s not found", topic)
// 	} 
// }
type MsgHandle func([]byte) (any, error)