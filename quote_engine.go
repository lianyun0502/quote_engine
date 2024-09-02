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
	WsAgent IWsAgent

	DoneSignal chan *struct{}
}


func NewQuoteEngine(cfg *Config) *QuoteEngine {
	// var wsAgent IWsAgent

	logger := logrus.New()
	InitLogger(logger, cfg)
	errHandle := WithErrorHandler(logger)
	engine := &QuoteEngine{
		Logger: logger,
		DoneSignal: make(chan *struct{}),
	}

	for _, wsCfg := range cfg.Websocket {
		switch strings.ToUpper(cfg.Websocket[0].Exchange) {
		// case "BINANCE":
		// 	msgHandle := WithBinanceMessageHandler(&wsCfg, logger)
		// 	wsAgent := exchange_conn.NewWebSocketAgent(binance_conn.NewWsClient(msgHandle, errHandle, wsCfg.ReconnTime))
		// 	wsAgent.Connect(wsCfg.Url)
		// 	wsAgent.Client.Logger = logger
		// 	engine.WsAgent = wsAgent
		case "BYBIT":
			msgHandle := WithBybitMessageHandler(&wsCfg, logger)
			wsAgent := exchange_conn.NewWebSocketAgent(bybit_conn.NewWsClient(msgHandle, errHandle, wsCfg.ReconnTime))
			wsAgent.Connect(wsCfg.Url)
			wsAgent.Client.Logger = logger
			engine.WsAgent = wsAgent
			go func(){
				for {
					<-wsAgent.Client.StartSignal
					subs, _ := json.Marshal(wsCfg.Subscribe)
					wsAgent.Subscribe(string(subs))
				}
			}()

		}
	}
	return engine
}


func WithErrorHandler(logger *logrus.Logger) func(error) {
	return func(err error) {
		logger.Error(err)
	}
}



func InitLogger(logger *logrus.Logger, config *Config) (err error){
	logger.SetReportCaller(config.Log.ReportCaller)
	writers := make(map[string]*rotatelogs.RotateLogs)
	logger.Info(common.PrettyPrint(config))
	shm.Logger = logger

	for _, writer := range config.Log.Writers {
		writers[writer.Name], _ = rotatelogs.New(
			config.Log.Dir + writer.Path,
			rotatelogs.WithLinkName(config.Log.LinkName),
			rotatelogs.WithMaxAge(time.Duration(writer.MaxAge)*24*time.Hour),
			rotatelogs.WithRotationTime(time.Duration(writer.RotationTime)*time.Hour),
		)
	}

	writeMap := make(lfshook.WriterMap)

	for key, writer := range config.Log.WriteMap {
		level, err := logrus.ParseLevel(key)
		if err != nil {
			return err
		}
		writeMap[level] = writers[writer]
	}

	hook := lfshook.NewHook(writeMap, &logrus.JSONFormatter{TimestampFormat: config.Log.Format})
	logger.SetFormatter(&logrus.TextFormatter{TimestampFormat: config.Log.Format, FullTimestamp: true})
	logger.AddHook(hook)
	level, err := logrus.ParseLevel(config.Log.Level)
	if err != nil {
		return err
	}
	logger.SetLevel(level)
	return nil
}