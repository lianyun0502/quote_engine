package quote_engine

import (
	// "encoding/json"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"regexp"
	"sort"
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
	// Luanch()
	SetSubscribeInstruments()
	SubscribeQuotes(quotes []string)
	UnsubscribeQuotes(quotes []string)
	GetSubscriptions() []string
}

type WsClient struct {
	WsClient *bybit_ws.WsBybitClient
	Quotes map[string]Quote
}

type QuoteEngine[WS IWsAgent] struct {
	Logger       *logrus.Logger
	Ws           map[string]WS
	Api          *bybit_http.ByBitClient
	DoneSignal   chan struct{}
	SubscribeMap map[string][]string    // map[Coin] [] subscribe topics
	SubscribeIns map[string]*instrument // map[Coin] instrument infomation
	Scheduler    map[string]*time.Ticker
	Cfg          *configs.WsClientConfig
	WsPool       *xsync.MapOf[string, *WsClient]
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
		// if spot.MarginTrade != "both" || spot.MarginTrade != "utaOnly" {
		// 	continue
		// }
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
	return ins
}

type CpStruct struct {
	Coin string
	Vol  float64
	Ins  *instrument
}

func (qe *QuoteEngine[WS]) MatchFilter(ins map[string]*instrument) map[string]*instrument {
	qe.Logger.Info("========= First 30 VOL coin =========")
	InsSlice := make([]*CpStruct, 0)
	for k, v := range ins {
		tickers, err := qe.Api.Market_Tickers("linear", bybit_http.WithQuery(map[string]string{"symbol": v.Perp.Symbol}))
		if err != nil {
			qe.Logger.Error(err)
		}

		if len(tickers) == 0 {
			delete(ins, k)
			continue
		}
		// fr, _ := strconv.ParseFloat(tickers[0].FR, 64)
		vol, _ := strconv.ParseFloat(tickers[0].Turnover24h, 64)
		InsSlice = append(InsSlice, &CpStruct{
			Coin: k,
			Vol:  vol,
			Ins:  v,
		})
	}
	sort.Slice(InsSlice, func(i, j int) bool {
		return InsSlice[i].Vol > InsSlice[j].Vol
	})
	retIns := make(map[string]*instrument)
	for i := 0; i < 30; i++ {
		retIns[InsSlice[i].Coin] = InsSlice[i].Ins
		qe.Logger.Infof("%s, Vol : %f", InsSlice[i].Coin, InsSlice[i].Vol)
	}
	qe.Logger.Infof("========= match =========")
	return retIns
}
func (qe *QuoteEngine[WS]) GetSubscribeInstruments(newInstruments map[string]*instrument) map[string]*instrument {
	ins := make(map[string]*instrument)
	for k, v := range newInstruments {
		if _, ok := qe.SubscribeIns[k]; !ok {
			ins[k] = v
			qe.SubscribeIns[k] = v
		}
	}
	return ins
}
func (qe *QuoteEngine[WS]) GetUnsubscribeInstruments(newInstruments map[string]*instrument) map[string]*instrument {
	ins := make(map[string]*instrument)
	for k := range qe.SubscribeIns {
		if _, ok := newInstruments[k]; !ok {
			ins[k] = qe.SubscribeIns[k]
			delete(qe.SubscribeIns, k)
		}
	}
	return ins
}
func (qe *QuoteEngine[WS]) UpdateSubscribeMap() {
	newInstruments := qe.MatchFilter(qe.GetInstruments())
	addMap := NewSubscribeMap2(qe.GetSubscribeInstruments(newInstruments), qe.Cfg.HostType)
	subMap := NewSubscribeMap2(qe.GetUnsubscribeInstruments(newInstruments), qe.Cfg.HostType)
	for k, v := range addMap {
		qe.SubscribeMap[k] = v
	}
	for k := range subMap {
		delete(qe.SubscribeMap, k)
	}
	qe.Subscribes(addMap)
	qe.Unsubscribes(subMap)
}

func (qe *QuoteEngine[WS]) IsCoinInPool(coin string) (bool, *WsClient) {
	var ws *WsClient
	f := func(k string, v *WsClient) bool {
		if _, ok := v.Quotes[coin]; ok {
			ws = v
			return false
		}
		return true
	}
	qe.WsPool.Range(f)
	if ws  != nil {
		return true, ws
	}
	return false, nil
}

func (qe *QuoteEngine[WS]) GetLeastLoadedWS() string {
	minSubscriptions := int(^uint(0) >> 1) // Max int value
	var wsID string
	qe.WsPool.Range(func(k string, v *WsClient) bool {
		if len(v.Quotes) < minSubscriptions {
			minSubscriptions = len(v.Quotes)
			wsID = k
		}
		return true
	})
	return wsID
}

func (qe *QuoteEngine[WS]) SubscribeQuotes(coins []string) {
	for _, coin := range coins {
		if ok, _ := qe.IsCoinInPool(coin); ok {
			qe.Logger.Warnf("Coin %s already subscribed", coin)
			continue
		}
		topics := GetSubscribeTopics(qe.SubscribeIns, coin, qe.Cfg.HostType)
		fmt.Println(topics)
		id := qe.GetLeastLoadedWS()
		ws, _ := qe.WsPool.Load(id)
		_, err := ws.WsClient.Subscribe(topics)
		if err != nil {
			qe.Logger.Error(err)
			continue
		}
		var symbol string
		switch qe.Cfg.HostType {
		case "spot":
			symbol = qe.SubscribeIns[coin].Spot.Symbol
		case "future":
			symbol = qe.SubscribeIns[coin].Perp.Symbol
		}
		// qe.Logger.Info(string(resp))
		ws.Quotes[coin] = Quote{
			Coin:   coin,
			Symbol: symbol,
			Topics: topics,
		}
	}
	qe.SaveSubscribes()
}
func (qe *QuoteEngine[WS]) UnsubscribeQuotes(coins []string) {
	for _, coin := range coins {
		ok, ws := qe.IsCoinInPool(coin)
		if !ok { 
			qe.Logger.Warnf("Coin %s not subscribed", coin)
			continue
		}
		topics := GetSubscribeTopics(qe.SubscribeIns, coin, qe.Cfg.HostType)
		fmt.Println(topics)
		_, err := ws.WsClient.Unsubscribe(topics)
		if err != nil {
			qe.Logger.Error(err)
			continue
		}
		// qe.Logger.Info(string(resp))
		delete(ws.Quotes, coin)
	}
	qe.SaveSubscribes()
}
func (qe *QuoteEngine[WS]) Subscribes(sub map[string][]string) {
	for k, v := range sub {
		i := nameToNumber(k, qe.Cfg.WsPoolSize)
		qe.Logger.Infof("subscribe(%d) %s %s", i, k, qe.Cfg.HostType)
		qe.Ws[strconv.FormatInt(int64(i), 10)].Subscribe(v)
	}
}
func (qe *QuoteEngine[WS]) Unsubscribes(sub map[string][]string) {
	for k, v := range sub {
		i := nameToNumber(k, qe.Cfg.WsPoolSize)
		qe.Logger.Infof("unsubscribe(%d) %s %s", i, k, qe.Cfg.HostType)
		qe.Ws[strconv.FormatInt(int64(i), 10)].Unsubscribe(v)
	}
}
func (qe *QuoteEngine[WS]) GetSubscriptions() []string{
	var subs []string
	qe.WsPool.Range(func(k string, v *WsClient) bool {
		for _, sub := range v.Quotes {
			subs = append(subs, sub.Coin)
		}
		return true
	})
	return subs
}
func (qe *QuoteEngine[WS]) SetSubscribeInstruments() {
	fmt.Println("SetSubscribeInstruments")
	qe.SubscribeIns = qe.GetInstruments()
	subscribtions, err  := qe.LoadSubscribes()
	if err == nil {
		if len(subscribtions) > 0 {
			qe.Logger.Info("Load subscribes from file")
			for _, v := range subscribtions {
				sub := make([]string, 0)
				for _, quote := range v {
					// fmt.Printf("%+v\n", quote)
					sub = append(sub, quote.Coin)
				}
				qe.SubscribeQuotes(sub)
			}
			return
		}
	}else{
		qe.Logger.Warning(err)
	}
	insFisrt30 := qe.MatchFilter(qe.SubscribeIns)
	for k := range insFisrt30 {
		qe.SubscribeQuotes([]string{k})
	}
}
func (qe *QuoteEngine[WS]) AddScheduler(name string, duration time.Duration, f func()) {
	ticker := time.NewTicker(duration)
	qe.Scheduler[name] = ticker
	go func() {
		for range ticker.C {
			f()
		}
	}()
}

type Quote struct {
	Coin   string   `json:"coin"`
	Symbol string   `json:"symbol"`
	Topics []string `json:"topics"`
}

func (qe *QuoteEngine[WS]) LoadSubscribes() (map[string][]*Quote, error) {
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

func (qe *QuoteEngine[WS]) SaveSubscribes() {
	file, err := os.OpenFile("subscribes.json", os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		qe.Logger.Error(err)
		return
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	subscribe := make(map[string][]*Quote)
	qe.WsPool.Range(func(k string, v *WsClient) bool {
		for _, sub := range v.Quotes {
			if _, ok := subscribe[k]; !ok {
				subscribe[k] = make([]*Quote, 0)
			}
			subscribe[k] = append(subscribe[k], &sub)
		}
		return true
	})
	err = encoder.Encode(subscribe)
	if err != nil {
		qe.Logger.Error(err)
	}
}

func nameToNumber(name string, num int) int {
	name = strings.ToLower(name)
	sum := 0
	for _, char := range name {
		sum += int(char - 'a' + 1)
	}

	return int(math.Abs(float64(sum))) % num
}
func NewBybitQuoteEngine(cfg *configs.WsClientConfig, logger *logrus.Logger) *QuoteEngine[*bybit_ws.WsBybitClient] {
	publisher_map := NewPublisherMap(cfg.Publisher)
	// engine := new(QuoteEngine[*bybit_ws.WsBybitClient])
	engine := &QuoteEngine[*bybit_ws.WsBybitClient]{
		Logger: logger,
		Ws:     make(map[string]*bybit_ws.WsBybitClient),
		Api:    bybit_http.NewSpotClient("", ""),
		Cfg:    cfg,
		WsPool: xsync.NewMapOf[string, *WsClient](),
	}
	// engine.Scheduler = make(map[string]*time.Ticker)
	for i := 0; i < cfg.WsPoolSize; i++ {
		handle := WithBybitMessageHandler2(cfg, logger, publisher_map)
		ws, err := bybit_ws.NewWsQuoteClient(cfg.HostType, handle)
		ws.PTimeout = 10
		if err != nil {
			logger.Error(err)
			continue
		}
		WsClinet := &WsClient{
			WsClient: ws,
			Quotes:   make(map[string]Quote),
		}
		engine.WsPool.Store(
			strconv.FormatInt(int64(i), 10), 
			WsClinet,
		)
		ws.Logger = logger
		ws.Connect()
		go WithResetWsFunc(WsClinet, engine.WsPool)()
		go ws.StartLoop()
	}
	return engine
}
// func WithResetSubScribeFunc(qe IQuoteEngine) func() chan  {
// 	resetCh := make(chan struct{})
// 	go func () {
// 		for range resetCh {
// 			qe.SubscribeCoins(qe.GetSubscriptions())
// 	return func() {
// 		for range qe.DoneSignal {
// 			qe.UpdateSubscribeMap()
// 		}
// 	}
// }

func WithResetWsFunc(ws *WsClient, wsPool *xsync.MapOf[string, *WsClient]) func() {
	return func() {
		for range ws.WsClient.StartSignal{
			var subs []string
			for _, quote := range ws.Quotes {
				subs = append(subs, quote.Topics...)
			}
			ws.WsClient.Subscribe(subs)
		}
	}
}

func NewQuoteEngine(cfg *configs.WsClientConfig, logger *logrus.Logger) any {
	switch strings.ToUpper(cfg.Exchange) {
	case "BYBIT":
		return NewBybitQuoteEngine(cfg, logger)
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

func GetSubscribeTopics(subscribes map[string]*instrument, coin string, category string) []string {
	topics := make([]string, 0)
	if ins, ok := subscribes[coin]; ok {
		switch category {
		case "spot":
			topics = append(topics, fmt.Sprintf("orderbook.50.%s", ins.Spot.Symbol))
			topics = append(topics, fmt.Sprintf("tickers.%s", ins.Spot.Symbol))
		case "future":
			topics = append(topics, fmt.Sprintf("orderbook.50.%s", ins.Perp.Symbol))
			topics = append(topics, fmt.Sprintf("tickers.%s", ins.Perp.Symbol))
		}
	}
	return topics
}
func NewSubscribeMap2(subscribes map[string]*instrument, category string) map[string][]string {
	sub_map := make(map[string][]string)
	for k, ins := range subscribes {
		if _, ok := sub_map[k]; !ok {
			sub_map[k] = make([]string, 0)
			switch category {
			case "spot":
				sub_map[k] = append(sub_map[k], fmt.Sprintf("orderbook.50.%s", ins.Spot.Symbol))
				sub_map[k] = append(sub_map[k], fmt.Sprintf("tickers.%s", ins.Spot.Symbol))
			case "future":
				sub_map[k] = append(sub_map[k], fmt.Sprintf("orderbook.50.%s", ins.Perp.Symbol))
				sub_map[k] = append(sub_map[k], fmt.Sprintf("tickers.%s", ins.Perp.Symbol))
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
