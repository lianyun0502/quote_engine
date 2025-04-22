package quote_engine

import (
	"encoding/json"
	// "fmt"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/lianyun0502/quote_engine/configs"
	"github.com/lianyun0502/exchange_conn/v2/common"
	"github.com/lianyun0502/shm"
	"github.com/rifflock/lfshook"
	"github.com/sirupsen/logrus"
	"github.com/puzpuzpuz/xsync/v3"
)

type IWsAgent interface {
	Send([]byte) error
	Subscribe([]string) ([]byte, error)
	Unsubscribe([]string) ([]byte, error)
	Connect() (*http.Response, error)
	StartLoop()
	Stop() error
}

type IQuoteEngine interface {
	Subscribe([]string) error
	Unsubscribe([]string) error
	SetSubscribeInstruments()
	GetSubscriptions() []string
}

type CpStruct[Info any] struct {
	Coin string
	Vol  float64
	Ins  *Instrument[Info]
}

type Quote struct {
	Coin   string   `json:"coin"`
	Symbol string   `json:"symbol"`
	Topics []string `json:"topics"`
}

type WsClient[WS any] struct {
	WsClient *WS
	Quotes map[string]Quote
}
type Instrument[Info any] struct {
	Scale int
	Perp  *Info
	Spot  *Info
}


func nameToNumber(name string, num int) int {
	name = strings.ToLower(name)
	sum := 0
	for _, char := range name {
		sum += int(char - 'a' + 1)
	}

	return int(math.Abs(float64(sum))) % num
}

// SetSubscribeInstruments 設定要訂閱的合約
func GetSubscribeInstruments[Info any](subscribeIns, newInstruments map[string]*Info) map[string]*Info {
	ins := make(map[string]*Info)
	for k, v := range newInstruments {
		if _, ok := subscribeIns[k]; !ok {
			ins[k] = v
			subscribeIns[k] = v
		}
	}
	return ins
}
// GetUnsubscribeInstruments 找出要取消訂閱的合約
func GetUnsubscribeInstruments[Info any](subscribeIns, newInstruments map[string]*Info) map[string]*Info {
	ins := make(map[string]*Info)
	for k := range subscribeIns {
		if _, ok := newInstruments[k]; !ok {
			ins[k] = subscribeIns[k]
			delete(subscribeIns, k)
		}
	}
	return ins
}

// IsCoinInPool 檢查某個幣是否在某個 pool 裡面
func IsCoinInPool[WS any](pool *xsync.MapOf[string, *WsClient[WS]], coin string) (bool, *WsClient[WS]) {
	var ws *WsClient[WS]
	f := func(k string, v *WsClient[WS]) bool {
		if _, ok := v.Quotes[coin]; ok {
			ws = v
			return false
		}
		return true
	}
	pool.Range(f)
	if ws  != nil {
		return true, ws
	}
	return false, nil
}

// GetLeastLoadedWS 找出訂閱數量最少的 WebSocket ID
func GetLeastLoadedWS[WS any](pool *xsync.MapOf[string, *WsClient[WS]]) string {
	minSubscriptions := int(^uint(0) >> 1) // Max int value
	var wsID string
	pool.Range(func(k string, v *WsClient[WS]) bool {
		if len(v.Quotes) < minSubscriptions {
			minSubscriptions = len(v.Quotes)
			wsID = k
		}
		return true
	})
	return wsID
}

func NewQuoteEngine(cfg *configs.WsClientConfig, logger *logrus.Logger) IQuoteEngine {
	switch strings.ToUpper(cfg.Exchange) {
	case "BYBIT":
		return NewBybitQuoteEngine(cfg, logger)
	case "BINANCE":
		return NewBinanceQuoteEngine(cfg, logger)
	default:
		logger.Error("Exchange not supported")
		return nil
	}
}
func WithSubscribeFunc(startSignal chan struct{}, wsAgent IWsAgent, subscribeList []string) func() {
	return func() {
		for range startSignal {
			for i := range subscribeList {
				wsAgent.Subscribe(subscribeList[i : i+1])
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

func SaveSubscribes[WS any](pool *xsync.MapOf[string, *WsClient[WS]]) error {
	file, err := os.OpenFile("subscribes.json", os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil { return err }
	defer file.Close()
	encoder := json.NewEncoder(file)
	subscribe := make(map[string][]*Quote)
	pool.Range(func(k string, v *WsClient[WS]) bool {
		for _, sub := range v.Quotes {
			if _, ok := subscribe[k]; !ok {
				subscribe[k] = make([]*Quote, 0)
			}
			subscribe[k] = append(subscribe[k], &sub)
		}
		return true
	})
	err = encoder.Encode(subscribe)
	if err != nil { return err }
	return nil
	
}

func LoadSubscribes() (map[string][]*Quote, error) {
	file, err := os.OpenFile("subscribes.json", os.O_RDONLY, 0666)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	quotes := make(map[string][]*Quote)
	err = decoder.Decode(&quotes)
	if err != nil {
		return nil, err
	}
	return quotes, nil
}

func GetSubscriptions[WS any](pool *xsync.MapOf[string, *WsClient[WS]]) []string{
	var subs []string
	pool.Range(func(k string, v *WsClient[WS]) bool {
		for _, sub := range v.Quotes {
			subs = append(subs, sub.Coin)
		}
		return true
	})
	return subs
}