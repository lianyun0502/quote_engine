package quote_engine

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/lianyun0502/exchange_conn/v1"

	// "github.com/lianyun0502/exchange_conn/v1/binance_conn"
	"github.com/lianyun0502/exchange_conn/v1/bybit_conn"
	"github.com/lianyun0502/exchange_conn/v1/common"
	"github.com/lianyun0502/shm"
	"github.com/rifflock/lfshook"
	"github.com/sirupsen/logrus"
)

type IWsAgent interface {
	Subscribe (string)
	Send ([]byte) error
	Connect (string) (*http.Response, error)
	StartLoop ()
	Stop () error
}

type QuoteEngine struct {
	Logger *logrus.Logger
	WsAgent map[string]IWsAgent

	DoneSignal chan *struct{}
}



func NewQuoteEngine(cfg *Config) *QuoteEngine {
	// var wsAgent IWsAgent
	logger := logrus.New()
	InitLogger(logger, &cfg.Log)
	errHandle := WithErrorHandler(logger)
	engine := &QuoteEngine{
		Logger: logger,
		DoneSignal: make(chan *struct{}),
		WsAgent: make(map[string]IWsAgent),
	}

	for _, wsCfg := range cfg.Websocket {
		pub_map := NewPublisherMap(wsCfg.Publisher)
		sub_map := NewSubscribeMap(wsCfg.Subscribe)
		switch strings.ToUpper(wsCfg.Exchange) {
		// case "BINANCE":
		// 	msgHandle := WithBinanceMessageHandler(&wsCfg, logger)
		// 	wsAgent := exchange_conn.NewWebSocketAgent(binance_conn.NewWsClient(msgHandle, errHandle, wsCfg.ReconnTime))
		// 	wsAgent.Connect(wsCfg.Url)
		// 	wsAgent.Client.Logger = logger
		// 	engine.WsAgent = wsAgent
		case "BYBIT":
			for k, v := range sub_map {
				logger.Infof("ws agent for %s started", k)
				msgHandle := WithBybitMessageHandler(&wsCfg, logger, pub_map)
				wsAgent := exchange_conn.NewWebSocketAgent(bybit_conn.NewWsClient(msgHandle, errHandle, wsCfg.ReconnTime))
				wsAgent.Connect(wsCfg.Url)
				wsAgent.Client.Logger = logger
				engine.WsAgent[k] = wsAgent
				go func(){
					// subscribe to topics everytime when connection is established
					for _ = range wsAgent.Client.StartSignal {
						subs, _ := json.Marshal(v)
						wsAgent.Subscribe(string(subs))
					}
				}()
				go wsAgent.StartLoop()
				
			}
		}
	}
	return engine
}


func WithErrorHandler(logger *logrus.Logger) func(error) {
	return func(err error) {
		logger.Error(err)
	}
}



func InitLogger(logger *logrus.Logger, config *LogConfig) (err error){
	logger.SetReportCaller(config.ReportCaller)
	writers := make(map[string]*rotatelogs.RotateLogs)
	logger.Info(common.PrettyPrint(config))
	shm.Logger = logger

	for _, writer := range config.Writers {
		writers[writer.Name], _ = rotatelogs.New(
			config.Dir + writer.Path,
			rotatelogs.WithLinkName(config.Dir + config.LinkName),
			rotatelogs.WithMaxAge(time.Duration(writer.MaxAge)*24*time.Hour),
			rotatelogs.WithRotationTime(time.Duration(writer.RotationTime)*time.Hour),
		)
	}

	writeMap := make(lfshook.WriterMap)

	for key, writer := range config.WriteMap {
		level, err := logrus.ParseLevel(key)
		if err != nil {
			return err
		}
		writeMap[level] = writers[writer]
	}

	hook := lfshook.NewHook(writeMap, &logrus.JSONFormatter{TimestampFormat: config.Format})
	logger.SetFormatter(&logrus.TextFormatter{TimestampFormat: config.Format, FullTimestamp: true})
	logger.AddHook(hook)
	level, err := logrus.ParseLevel(config.Level)
	if err != nil {
		return err
	}
	logger.SetLevel(level)
	return nil
}

func NewSubscribeMap(subscribes []string) map[string][]string {
	sub_map := make(map[string][]string)
		for _, sub := range subscribes {
			symbols := strings.Split(sub, ".")
			symbol := symbols[len(symbols)-1]
			if _, ok := sub_map[symbol]; !ok {
				sub_map[symbol] = []string{sub}
			}else{
				sub_map[symbol] = append(sub_map[symbol], sub)
			}
		}
	return sub_map

}

func NewPublisherMap(publishers []PublisherConfig) map[string]*shm.Publisher {
	pub_map := make(map[string]*shm.Publisher)	
		for _, pub := range publishers {
			pub_map[pub.Topic] = shm.NewPublisher(pub.Skey, pub.Size)
		}
	return pub_map
}

func NewSubscriberMap(publishers []PublisherConfig) map[string]*shm.Subscriber{
	sub_map := make(map[string]*shm.Subscriber)	
		for _, pub := range publishers {
			sub_map[pub.Topic] = shm.NewSubscriber(pub.Skey, pub.Size)
		}
	return sub_map
}