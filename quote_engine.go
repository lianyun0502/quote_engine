package quote_engine

import (
	// "encoding/json"
	"net/http"
	"strings"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	// "github.com/lianyun0502/exchange_conn/v2/ws_client"
	"github.com/lianyun0502/quote_engine/configs"

	"github.com/lianyun0502/exchange_conn/v2/bybit/ws_client"
	"github.com/lianyun0502/exchange_conn/v2/common"
	"github.com/lianyun0502/shm"
	"github.com/rifflock/lfshook"
	"github.com/sirupsen/logrus"
)

type IWsAgent interface {
	Send([]byte) error
	Subscribe([]string) ([]byte, error)
	Connect() (*http.Response, error)
	StartLoop()
	Stop() error
}

type IQuoteEngine interface {
	Luanch()
}

type QuoteEngine[WS any] struct {
	Logger       *logrus.Logger
	Ws           map[string]*WS
	DoneSignal   chan struct{}
	SubscribeMap map[string][]string
}


func NewQuoteEngine(cfg *configs.WsClientConfig, logger *logrus.Logger) (engine IQuoteEngine) {
	publisher_map := NewPublisherMap(cfg.Publisher)
	switch strings.ToUpper(cfg.Exchange) {
	case "BYBIT":
		engine := new(QuoteEngine[bybit.WsBybitClient])
		// engine.Logger = logger
		engine.Ws = make(map[string]*bybit.WsBybitClient)
		// engine.DoneSignal = make(chan struct{})
		engine.SubscribeMap = NewSubscribeMap(cfg.Subscribe)
		for k, _ := range engine.SubscribeMap {
			logger.Infof("ws agent for %s started", k)
			handle := WithBybitMessageHandler(cfg, logger, publisher_map)
			ws, err := bybit.NewWsQuoteClient(cfg.HostType, handle)
			if err != nil {
				logger.Error(err)
				continue
			}
			engine.Ws[k] = ws
			ws.Logger = logger
			go func () {
				ws.Connect()
				go func () {
					for range ws.StartSignal {
						go ws.Subscribe(engine.SubscribeMap[k])
					}
				}() 
				go ws.StartLoop()	
			}()
		}

	default:
		logger.Error("Exchange not supported")
		return nil
	}
	return engine
}

func WithErrorHandler(logger *logrus.Logger) func(error) {
	return func(err error) {
		logger.Error(err)
	}
}

func InitLogger(logger *logrus.Logger, config *configs.LogConfig) (err error) {
	logger.SetReportCaller(config.ReportCaller)
	writers := make(map[string]*rotatelogs.RotateLogs)
	logger.Info(common.PrettyPrint(config))
	shm.Logger = logger

	for _, writer := range config.Writers {
		writers[writer.Name], _ = rotatelogs.New(
			config.Dir+writer.Path,
			rotatelogs.WithLinkName(config.Dir+config.LinkName),
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
		} else {
			sub_map[symbol] = append(sub_map[symbol], sub)
		}
	}
	return sub_map

}

func NewPublisherMap(publishers []configs.PublisherConfig) map[string]*shm.Publisher {
	pub_map := make(map[string]*shm.Publisher)
	for _, pub := range publishers {
		pub_map[pub.Topic] = shm.NewPublisher(pub.Skey, pub.Size)
	}
	return pub_map
}

func NewSubscriberMap(publishers []configs.PublisherConfig) map[string]*shm.Subscriber {
	sub_map := make(map[string]*shm.Subscriber)
	for _, pub := range publishers {
		sub_map[pub.Topic] = shm.NewSubscriber(pub.Skey, pub.Size)
	}
	return sub_map
}
