package quote_engine

import (
	"encoding/json"
	"regexp"
	"time"
	"strconv"
	"fmt"
	"sort"

	"github.com/lianyun0502/exchange_conn/v2/bybit/data_stream"
	"github.com/lianyun0502/quote_engine/configs"
	"github.com/lianyun0502/shm"
	bybit_http "github.com/lianyun0502/exchange_conn/v2/bybit/http_client"
	bybit_resp "github.com/lianyun0502/exchange_conn/v2/bybit/response"
	bybit_ws "github.com/lianyun0502/exchange_conn/v2/bybit/ws_client"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fastjson"
	"github.com/puzpuzpuz/xsync/v3"
)


type IWriter interface {
	Write([]byte)
}

// 將 bybit 的 symbol的綴詞 轉換成 topic，如果不符合任何規則，則回傳原本的 symbol
func ByBitSymbolToTopic(symbol string) string {
	switch {
	case regexp.MustCompile("Trade").MatchString(symbol):
		return "Trade"
	case regexp.MustCompile("orderbook").MatchString(symbol):
		return "orderbook"
	case regexp.MustCompile("ticker").MatchString(symbol):
		return "ticker"
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
		logger.Debug(string(rawData))
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

// 給定 WsClientConfig 和 logger 生成一個 bybit 的 message handler 的 closure,
func WithBybitMessageHandler2(wsCfg *configs.WsClientConfig, logger *logrus.Logger, pub_map map[string]*shm.Publisher) func([]byte) {
	// map of publisher, key is topic, value is publisher
	handleMap := make(map[string]func([]byte))
	for _, pub := range wsCfg.Publisher {
		topic := ByBitSymbolToTopic(pub.Topic)
		switch topic {
		case "Trade":
			handleMap[topic] = WithTradeHandler(logger, pub_map[topic])
		case "orderbook":
			handleMap[topic] = WithOrderBookHandler(logger, pub_map[topic])
		case "ticker":
			handleMap[topic] = WithTickerHandler(logger, pub_map[topic])
		default:
			logger.Errorf("can not gen handler for unknown topic: %s", topic)
		}
	}
	dataCH := make(chan []byte, 1000)
	go func() {
		for rawData := range dataCH {
			logger.Debugf("rev time: %d", time.Now().UnixNano()/int64(time.Millisecond))
			logger.Debug(string(rawData))
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
	}()
	return func(rawData []byte) {
		dataCH <- rawData
	}
}


type BybitQuoteEngine struct {
	Logger       *logrus.Logger
	Api          *bybit_http.ByBitClient
	DoneSignal   chan struct{}
	SubscribeMap map[string][]string       // map[Coin] [] subscribe topics
	SubscribeIns map[string]*Instrument[bybit_resp.InstrumentInfo] // map[Coin] instrument infomation
	Scheduler    map[string]*time.Ticker
	Cfg          *configs.WsClientConfig
	WsPool       *xsync.MapOf[string, *WsClient[bybit_ws.WsBybitClient]]
}


func NewBybitQuoteEngine(cfg *configs.WsClientConfig, logger *logrus.Logger) *BybitQuoteEngine {
	publisher_map := NewPublisherMap(cfg.Publisher)
	engine := &BybitQuoteEngine{
		Logger: logger,
		Api:    bybit_http.NewSpotClient("", ""),
		Cfg:    cfg,
		WsPool: xsync.NewMapOf[string, *WsClient[bybit_ws.WsBybitClient]](),
		SubscribeIns: make(map[string]*Instrument[bybit_resp.InstrumentInfo]),
		SubscribeMap: make(map[string][]string),
	}
	for i := 0; i < cfg.WsPoolSize; i++ {
		handle := WithBybitMessageHandler2(cfg, logger, publisher_map)
		ws, err := bybit_ws.NewWsQuoteClient(cfg.HostType, handle)
		ws.PTimeout = 10
		if err != nil {
			logger.Error(err)
			continue
		}
		WsClinet := &WsClient[bybit_ws.WsBybitClient]{
			WsClient: ws,
			Quotes:   make(map[string]Quote),
		}
		engine.WsPool.Store(
			strconv.FormatInt(int64(i), 10), 
			WsClinet,
		)
		ws.Logger = logger
		ws.PostStartFunc = WithPostStartFunc(ws, WsClinet.Quotes)
		ws.Connect()
		// go WithResetWsFunc(WsClinet, engine.WsPool)()
		// go ws.StartLoop()
	}
	return engine
}

func WithPostStartFunc(ws *bybit_ws.WsBybitClient, Quotes map[string]Quote) func() error{
	return func() error {
		topic_list := make([]string, 0)
		for _, quote := range Quotes {
			topic_list = append(topic_list, quote.Topics...)
		}
		_, err := ws.Subscribe(topic_list)
		if err != nil {
			return err
		}
		return nil
	}
}

// 獲得有現貨和永續合約的幣種
func (qe *BybitQuoteEngine) GetInstruments() map[string]*Instrument[bybit_resp.InstrumentInfo] {
	spots, _ := qe.Api.Market_InstrumentsInfo("spot")
	ins := make(map[string]*Instrument[bybit_resp.InstrumentInfo])
	for _, spot := range spots {
		if spot.QuoteCoin != "USDT" || spot.Status != "Trading" {
			continue
		}
		// if spot.MarginTrade != "both" || spot.MarginTrade != "utaOnly" {
		// 	continue
		// }
		ins[spot.BaseCoin] = &Instrument[bybit_resp.InstrumentInfo]{
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

func (qe *BybitQuoteEngine) GetSubscribeTopics( coin string, category string) []string {
	topics := make([]string, 0)
	if ins, ok := qe.SubscribeIns[coin]; ok {
		switch category {
		case "spot":
			topics = append(topics, fmt.Sprintf("orderbook.50.%s", ins.Spot.Symbol))
			topics = append(topics, fmt.Sprintf("tickers.%s", ins.Spot.Symbol))
			topics = append(topics, fmt.Sprintf("publicTrade.%s", ins.Spot.Symbol))
		case "future":
			topics = append(topics, fmt.Sprintf("orderbook.50.%s", ins.Perp.Symbol))
			topics = append(topics, fmt.Sprintf("tickers.%s", ins.Perp.Symbol))
			topics = append(topics, fmt.Sprintf("publicTrade.%s", ins.Spot.Symbol))
		}
	}
	return topics
}

func (qe *BybitQuoteEngine) Subscribe(coins []string) error {
	for _, coin := range coins {
		if ok, _ := IsCoinInPool(qe.WsPool, coin); ok {
			qe.Logger.Warnf("Coin %s already subscribed", coin)
			continue
		}
		topics := qe.GetSubscribeTopics(coin, qe.Cfg.HostType)
		fmt.Println(topics)
		id := GetLeastLoadedWS(qe.WsPool)
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
	SaveSubscribes(qe.WsPool)
	return nil
}

func (qe *BybitQuoteEngine) Unsubscribe(coins []string) error{
	for _, coin := range coins {
		ok, ws := IsCoinInPool(qe.WsPool, coin)
		if !ok { 
			qe.Logger.Warnf("Coin %s not subscribed", coin)
			continue
		}
		topics := qe.GetSubscribeTopics(coin, qe.Cfg.HostType)
		fmt.Println(topics)
		_, err := ws.WsClient.Unsubscribe(topics)
		if err != nil {
			qe.Logger.Error(err)
			continue
		}
		delete(ws.Quotes, coin)
	}
	SaveSubscribes(qe.WsPool)
	return nil
}

func (qe *BybitQuoteEngine) SetSubscribeInstruments() {
	fmt.Println("SetSubscribeInstruments")
	qe.SubscribeIns = qe.GetInstruments()
	subscribtions, err  := LoadSubscribes()
	if err == nil {
		if len(subscribtions) > 0 {
			qe.Logger.Info("Load subscribes from file")
			for _, v := range subscribtions {
				sub := make([]string, 0)
				for _, quote := range v {
					// fmt.Printf("%+v\n", quote)
					sub = append(sub, quote.Coin)
				}
				qe.Subscribe(sub)
			}
			return
		}
	}else{
		qe.Logger.Warning(err)
	}
	insFisrt30 := qe.MatchFilter(qe.SubscribeIns)
	for k := range insFisrt30 {
		qe.Subscribe([]string{k})
	}
}


func (qe *BybitQuoteEngine) MatchFilter(ins map[string]*Instrument[bybit_resp.InstrumentInfo]) map[string]*Instrument[bybit_resp.InstrumentInfo] {
	qe.Logger.Info("========= First 30 VOL coin =========")
	InsSlice := make([]*CpStruct[bybit_resp.InstrumentInfo], 0)
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
		InsSlice = append(InsSlice, &CpStruct[bybit_resp.InstrumentInfo]{
			Coin: k,
			Vol:  vol,
			Ins:  v,
		})
	}
	sort.Slice(InsSlice, func(i, j int) bool {
		return InsSlice[i].Vol > InsSlice[j].Vol
	})
	retIns := make(map[string]*Instrument[bybit_resp.InstrumentInfo])
	for i := 0; i < 30; i++ {
		retIns[InsSlice[i].Coin] = InsSlice[i].Ins
		qe.Logger.Infof("%s, Vol : %f", InsSlice[i].Coin, InsSlice[i].Vol)
	}
	qe.Logger.Infof("========= match =========")
	return retIns
}

func (qe *BybitQuoteEngine) GetSubscriptions() []string{
	return GetSubscriptions(qe.WsPool)
}


