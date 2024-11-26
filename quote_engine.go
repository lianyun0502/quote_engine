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

type QuoteEngine struct {
	Logger  *logrus.Logger
	WsAgent map[string]IWsAgent

	DoneSignal chan struct{}
}

func NewQuoteEngine(cfg *configs.Config, logger *logrus.Logger) *QuoteEngine {
	// var wsAgent IWsAgent
	// errHandle := WithErrorHandler(logger)
	engine := &QuoteEngine{
		Logger:     logger,
		DoneSignal: make(chan struct{}),
		WsAgent:    make(map[string]IWsAgent),
	}

	for _, wsCfg := range cfg.Websocket {
		publisher_map := NewPublisherMap(wsCfg.Publisher)
		subscribe_map := NewSubscribeMap(wsCfg.Subscribe)

		switch strings.ToUpper(wsCfg.Exchange) {
		case "BYBIT":
			for k, v := range subscribe_map {
				logger.Infof("ws agent for %s started", k)
				msgHandle := WithBybitMessageHandler(&wsCfg, logger, publisher_map)
				ws, err := bybit.NewWsQuoteClient(wsCfg.HostType, msgHandle)
				if err != nil {
					logger.Error(err)
					continue
				}
				ws.Logger = logger
				engine.WsAgent[k] = ws
				ws.Connect()
				go ws.StartLoop()
				go func() {
					for range ws.StartSignal {
						ws.Subscribe(v)
					}
				}()
			}
		default:
			logger.Errorf("unsupported exchange: %s", wsCfg.Exchange)
		}
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

