package quote_engine

import (
	"encoding/json"
	"regexp"
	"time"

	"github.com/lianyun0502/exchange_conn/v1/bybit_conn/data_stream"
	"github.com/lianyun0502/quote_engine/configs"
	"github.com/lianyun0502/shm"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fastjson"
)

type IWriter interface {
	Write([]byte)
}

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

func WithTradeHandler(logger *logrus.Logger, writer IWriter) func([]byte) {
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
				logger.Debugf("exch send time: %d", data.Time)
				jsonData, err := json.Marshal(data)
				if err != nil {
					logger.Error(err)
					return
				}
				writer.Write(jsonData)
			}
		}
	}
}

func WithOrderBookHandler(logger *logrus.Logger, writer IWriter) func([]byte) {
	logger.Debug("orderbook handler")
	parser := data_stream.NewOrderBook(10)
	return func(rawData []byte) {
		data, err := parser.Update(rawData)
		if err != nil {
			logger.Debug(string(rawData))
			logger.Error(err)
			return
		}
		if data != nil {
			logger.Debugf("exch send time: %d", data.Time)
			jsonData, err := json.Marshal(data)
			if err != nil {
				logger.Error(err)
				return
			}
			writer.Write(jsonData)
		}
	}
}

func WithTickerHandler(logger *logrus.Logger, writer IWriter) func([]byte) {
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
			logger.Debugf("exch send time: %d", data.Time)
			jsonData, err := json.Marshal(data)
			if err != nil {
				logger.Error(err)
				return
			}
			writer.Write(jsonData)
		}
	}
}

// 給定 WsClientConfig 和 logger 生成一個 bybit 的 message handler 的 closure,
func WithBybitMessageHandler(wsCfg *configs.WsClientConfig, logger *logrus.Logger, pub_map map[string]*shm.Publisher) func([]byte) {
	// map of publisher, key is topic, value is publisher
	handleMap := make(map[string]func([]byte))
	for _, pub := range wsCfg.Publisher {
		topic := ByBitSymbolToTopic(pub.Topic)
		switch topic {
		case "publicTrade":
			handleMap[topic] = WithTradeHandler(logger, pub_map[topic])
		case "orderbook":
			handleMap[topic] = WithOrderBookHandler(logger, pub_map[topic])
		case "tickers":
			handleMap[topic] = WithTickerHandler(logger, pub_map[topic])
		default:
			logger.Errorf("can not gen handler for unknown topic: %s", topic)
		}
	}
	return func(rawData []byte) {
		logger.Debugf("rev time: %d", time.Now().UnixNano()/int64(time.Millisecond))
		// logger.Debug(string(rawData))
		v := fastjson.MustParseBytes(rawData)
		symbol := string(v.GetStringBytes("topic"))
		topic := ByBitSymbolToTopic(symbol)
		if handler, ok := handleMap[topic]; ok {
			logger.Debugf("rev topic: %s", topic)
			handler(rawData)
		} else {
			logger.Errorf("unknown topic: %s", topic)
		}
	}
}
