package quote_engine

import (
	// "encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/lianyun0502/quote_engine/configs"

	bybit_http "github.com/lianyun0502/exchange_conn/v2/bybit/http_client"
	bybit_resp "github.com/lianyun0502/exchange_conn/v2/bybit/response"
	bybit_ws "github.com/lianyun0502/exchange_conn/v2/bybit/ws_client"
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
	Api 		 *bybit_http.ByBitClient
	DoneSignal   chan struct{}
	SubscribeMap map[string][]string
	subscribeFunc map[string] func()
}

type instrument struct {
	Scale int
	Perp  *bybit_resp.InstrumentInfo
	Spot  *bybit_resp.InstrumentInfo
}


func (qe *QuoteEngine[WS]) GetInstruments() map[string]*instrument {
	spots, _ := qe.Api.Market_InstrumentsInfo("spot")
	ins := make(map[string]*instrument)
	for _, spot := range spots {
		if spot.QuoteCoin != "USDT" || spot.Status != "Trading" {
			continue
		}
		ins[spot.BaseCoin] = &instrument{
			Perp: nil,
			Spot: &spot,
		}
	}
	perps, _ := qe.Api.Market_InstrumentsInfo("linear")
	for _, perp := range perps {
		if perp.QuoteCoin != "USDT" || perp.ContractType != "LinearPerpetual" {
			continue
		}
		re := regexp.MustCompile(`(\d+)(\D+)`)
		matches := re.FindStringSubmatch(perp.BaseCoin)
		scale := int64(1)
		if len(matches) >= 2 {
			if matches[1] != "1" {
				perp.BaseCoin = matches[2]
				scale, _ = strconv.ParseInt(matches[1], 10, 64)
			}
		}
		if _, ok := ins[perp.BaseCoin]; ok {
			if ins[perp.BaseCoin].Perp != nil {
				qe.Logger.WithField("Coin", perp.BaseCoin).Warn("Perp is not nil")
			}
			ins[perp.BaseCoin].Scale = int(scale)
			ins[perp.BaseCoin].Perp = &perp
		}
	}
	for coin, v := range ins {
		if v.Perp == nil || v.Spot == nil {
			delete(ins, coin)
		}
	}

	// tickers, err := qe.Api.Market_Tickers("spot")
	// if err != nil {
	// 	qe.Logger.Error(err)
	// }
	// sort.Slice(tickers, func(i, j int) bool {
	// 	tickers[i].Symbol
	// 	return tickers[i].Symbol < tickers[j].Symbol
	// })
	// for _, ticker := range tickers {

		
	// }

	return ins
}


func NewQuoteEngine(cfg *configs.WsClientConfig, logger *logrus.Logger) (engine IQuoteEngine) {
	publisher_map := NewPublisherMap(cfg.Publisher)
	switch strings.ToUpper(cfg.Exchange) {
	case "BYBIT":
		engine := new(QuoteEngine[bybit_ws.WsBybitClient])
		engine.Logger = logger
		engine.subscribeFunc = make(map[string]func())
		engine.Ws = make(map[string]*bybit_ws.WsBybitClient)
		engine.Api = bybit_http.NewSpotClient("", "")
		ins := engine.GetInstruments()
		engine.SubscribeMap = NewSubscribeMap2(ins, cfg.HostType)
		i := 0
		subscribeList := make([][]string, 10)
		for k, v := range ins {
			i++
			logger.Infof("ws agent for %s started", k)
			logger.Infof("Perp: %s, Spot: %s", v.Perp.Symbol, v.Spot.Symbol)
			subscribeList[i%10] = append(subscribeList[i%10], engine.SubscribeMap[k]...)
		}
		for i := range subscribeList{
			handle := WithBybitMessageHandler(cfg, logger, publisher_map)
			ws, err := bybit_ws.NewWsQuoteClient(cfg.HostType, handle)
			if err != nil {
				logger.Error(err)
				continue
			}
			engine.Ws[string(i)] = ws
			ws.Logger = logger
			ws.Connect()
			engine.subscribeFunc[string(i)] = WithSubscribeFunc(ws.StartSignal, ws, subscribeList[i])
			go engine.subscribeFunc[string(i)]()
			go ws.StartLoop()
		}
	default:
		logger.Error("Exchange not supported")
		return nil
	}
	return engine
}

func WithSubscribeFunc(startSignal chan struct{}, wsAgent IWsAgent, subscribeList []string) func() {
	return func() {
		for range startSignal {
			for i, _ := range subscribeList {
				wsAgent.Subscribe(subscribeList[i:i+1])
			}
		}
	}
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
func NewSubscribeMap2(subscribes map[string]*instrument, category string) map[string][]string {
	sub_map := make(map[string][]string)
	for k, ins := range subscribes {
		if _, ok := sub_map[k]; !ok {
			sub_map[k] =make([]string, 0)
		switch category {
		case "spot":
			sub_map[k] = append(sub_map[k], fmt.Sprintf("orderbook.50.%s",ins.Spot.Symbol))
			sub_map[k] = append(sub_map[k], fmt.Sprintf("tickers.%s",ins.Spot.Symbol))
		case "future":
			sub_map[k] = append(sub_map[k], fmt.Sprintf("orderbook.50.%s",ins.Perp.Symbol))
			sub_map[k] = append(sub_map[k], fmt.Sprintf("tickers.%s",ins.Perp.Symbol))
			}
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
