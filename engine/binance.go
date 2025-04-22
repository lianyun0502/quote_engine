package quote_engine

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	// "github.com/lianyun0502/exchange_conn/v2/binance/data_stream"
	binance_http "github.com/lianyun0502/exchange_conn/v2/binance/http_client"
	"github.com/lianyun0502/exchange_conn/v2/binance/data_stream"
	"github.com/lianyun0502/quote_engine/configs"

	// binance_resp "github.com/lianyun0502/exchange_conn/v2/binance/response"
	binance_ws "github.com/lianyun0502/exchange_conn/v2/binance/ws_client"

	"github.com/lianyun0502/shm"
	"github.com/puzpuzpuz/xsync/v3"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fastjson"
)

func SymbolToTopic(symbol string) string {
	switch {
		case regexp.MustCompile(`aggTrade`).MatchString(symbol):
			return "aggTrade"
		case regexp.MustCompile(`trade`).MatchString(symbol):
			return "trade"
		case regexp.MustCompile(`orderbook`).MatchString(symbol):
			return "orderbook"
		case regexp.MustCompile(`ticker`).MatchString(symbol):
			return "ticker"
		default:
			return symbol
	}
}

// life time is infinity
func WithBinanceMessageHandler(wsCfg *configs.WsClientConfig, logger *logrus.Logger) func([]byte) {
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

func BinanceSymbolToTopic(symbol string) string {
	switch {
	case regexp.MustCompile("aggTrade").MatchString(symbol):
		return "Trade"
	case regexp.MustCompile("depthUpdate").MatchString(symbol):
		return "orderbook"
	case regexp.MustCompile("ticker").MatchString(symbol):
		return "ticker"
	case regexp.MustCompile("24hrTicker").MatchString(symbol):
		return "ticker"
	default:
		return "orderbook"
	}
}
func WithTradeHandler2(logger *logrus.Logger, writer IWriter) func([]byte) {
	logger.Debug("trade handler")
	// parser := data_stream.
	return func(rawData []byte) {
		data, err := data_stream.ToNormalAggregateTradeData(rawData)
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

func WithOrderBookHandler2(logger *logrus.Logger, writer IWriter, cfg *configs.WsClientConfig) func([]byte) {
	logger.Debug("orderbook handler")
	// parser := data_stream.NewPartialOrderBook(10)
	parser, _ := data_stream.NewOrderBookMap(cfg.HostType)
	return func(rawData []byte) {
		data, err := parser.Update(rawData)
		if err != nil {
			// logger.Debug(string(rawData))
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

func WithTickerHandler2(logger *logrus.Logger, writer IWriter) func([]byte) {
	logger.Debug("ticker handler")
	// parser := data_stream.NewMarketData()
	return func(rawData []byte) {
		data, err := data_stream.UpdateMarketPrice(rawData)
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
func WithBinanceMessageHandler2(wsCfg *configs.WsClientConfig, logger *logrus.Logger, pub_map map[string]*shm.Publisher) func([]byte) {
	// map of publisher, key is topic, value is publisher
	handleMap := make(map[string]func([]byte))
	for _, pub := range wsCfg.Publisher {
		topic := SymbolToTopic(pub.Topic)
		switch topic {
		case "aggTrade":
			handleMap[topic] = WithTradeHandler2(logger, pub_map[topic])
		case "Trade":
			handleMap[topic] = WithTradeHandler2(logger, pub_map[topic])
		case "orderbook":
			handleMap[topic] = WithOrderBookHandler2(logger, pub_map[topic], wsCfg)
		case "ticker":
			handleMap[topic] = WithTickerHandler2(logger, pub_map[topic])
		default:
			logger.Errorf("can not gen handler for unknown topic: %s", topic)
			// handleMap[topic] = WithOrderBookHandler2(logger, pub_map["depthUpdate"])
		}
	}
	dataCH := make(chan []byte, 1000)
	go func() {
		for rawData := range dataCH {
			logger.Debugf("rev time: %d", time.Now().UnixNano()/int64(time.Millisecond))
			logger.Debug(string(rawData))
			v := fastjson.MustParseBytes(rawData)
			symbol := string(v.GetStringBytes("e"))
			topic := BinanceSymbolToTopic(symbol)
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


type TempInstrumentInfo struct {
	Symbol string
}

type BinanceQuoteEngine struct {
	Logger       *logrus.Logger
	Api          *binance_http.BinanceClient
	DoneSignal   chan struct{}
	SubscribeMap map[string][]string       // map[Coin] [] subscribe topics
	SubscribeIns map[string]*Instrument[TempInstrumentInfo] // map[Coin] instrument infomation
	Scheduler    map[string]*time.Ticker
	Cfg          *configs.WsClientConfig
	WsPool       *xsync.MapOf[string, *WsClient[binance_ws.WsBinanceClient]]
}


func NewBinanceQuoteEngine(cfg *configs.WsClientConfig, logger *logrus.Logger) *BinanceQuoteEngine {
	publisher_map := NewPublisherMap(cfg.Publisher)
	api, _ := binance_http.NewAPISpotClient("", "")
	engine := &BinanceQuoteEngine{
		Logger: logger,
		Api:    api,
		Cfg:    cfg,
		WsPool: xsync.NewMapOf[string, *WsClient[binance_ws.WsBinanceClient]](),
		SubscribeIns: make(map[string]*Instrument[TempInstrumentInfo]),
		SubscribeMap: make(map[string][]string),
	}
	for i := 0; i < cfg.WsPoolSize; i++ {
		handle := WithBinanceMessageHandler2(cfg, logger, publisher_map)
		ws, err := binance_ws.NewWsQuoteClient(cfg.HostType, handle)
		ws.PTimeout = 10
		if err != nil {
			logger.Error(err)
			continue
		}
		WsClinet := &WsClient[binance_ws.WsBinanceClient]{
			WsClient: ws,
			Quotes:   make(map[string]Quote),
		}
		engine.WsPool.Store(
			strconv.FormatInt(int64(i), 10), 
			WsClinet,
		)
		ws.Logger = logger
		time.Sleep(1 * time.Second)
		ws.Connect()
		// go WithResetWsFunc(WsClinet, engine.WsPool)()
		go ws.StartLoop()
	}
	return engine
}


// 獲得有現貨和永續合約的幣種
// func (qe *BinanceQuoteEngine) GetInstruments() map[string]*Instrument[bybit_resp.InstrumentInfo] {
// 	spots, _ := qe.Api.Market_InstrumentsInfo("spot")
// 	ins := make(map[string]*Instrument[bybit_resp.InstrumentInfo])
// 	for _, spot := range spots {
// 		if spot.QuoteCoin != "USDT" || spot.Status != "Trading" {
// 			continue
// 		}
// 		// if spot.MarginTrade != "both" || spot.MarginTrade != "utaOnly" {
// 		// 	continue
// 		// }
// 		ins[spot.BaseCoin] = &Instrument[bybit_resp.InstrumentInfo]{
// 			Perp: nil,
// 			Spot: &spot,
// 		}
// 	}
// 	perps, _ := qe.Api.Market_InstrumentsInfo("linear")
// 	for _, perp := range perps {
// 		if perp.QuoteCoin != "USDT" || perp.ContractType != "LinearPerpetual" {
// 			continue
// 		}
// 		re := regexp.MustCompile(`(\d+)(\D+)`)
// 		matches := re.FindStringSubmatch(perp.BaseCoin)
// 		scale := int64(1)
// 		if len(matches) >= 2 {
// 			if matches[1] != "1" {
// 				perp.BaseCoin = matches[2]
// 				scale, _ = strconv.ParseInt(matches[1], 10, 64)
// 			}
// 		}
// 		if _, ok := ins[perp.BaseCoin]; ok {
// 			if ins[perp.BaseCoin].Perp != nil {
// 				qe.Logger.WithField("Coin", perp.BaseCoin).Warn("Perp is not nil")
// 			}
// 			ins[perp.BaseCoin].Scale = int(scale)
// 			ins[perp.BaseCoin].Perp = &perp
// 		}
// 	}
// 	for coin, v := range ins {
// 		if v.Perp == nil || v.Spot == nil {
// 			delete(ins, coin)
// 		}
// 	}
// 	return ins
// }

func (qe *BinanceQuoteEngine) GetSubscribeTopics( coin string, category string) []string {
	topics := make([]string, 0)
	if ins, ok := qe.SubscribeIns[coin]; ok {
		switch category {
		case "spot":
			topics = append(topics, fmt.Sprintf("%s@depth@100ms", strings.ToLower(ins.Spot.Symbol)))
			topics = append(topics, fmt.Sprintf("%s@aggTrade", strings.ToLower(ins.Spot.Symbol)))
			topics = append(topics, fmt.Sprintf("%s@ticker", strings.ToLower(ins.Spot.Symbol)))
		case "future":
			topics = append(topics, fmt.Sprintf("%s@depth@100ms", strings.ToLower(ins.Spot.Symbol)))
			topics = append(topics, fmt.Sprintf("%s@aggTrade", strings.ToLower(ins.Spot.Symbol)))
			topics = append(topics, fmt.Sprintf("%s@ticker", strings.ToLower(ins.Spot.Symbol)))
		}
	}
	return topics
}

func (qe *BinanceQuoteEngine) Subscribe(coins []string) error {
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

func (qe *BinanceQuoteEngine) Unsubscribe(coins []string) error{
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

func (qe *BinanceQuoteEngine) SetSubscribeInstruments() {
	fmt.Println("SetSubscribeInstruments")
	// qe.SubscribeIns = qe.GetInstruments()
	qe.SubscribeIns["BTC"] = &Instrument[TempInstrumentInfo]{
			Spot: &TempInstrumentInfo{Symbol: "BTCUSDT"},
			Perp: &TempInstrumentInfo{Symbol: "BTCUSDT"},
		}
	qe.SubscribeIns["ETH"] = &Instrument[TempInstrumentInfo]{
			Spot: &TempInstrumentInfo{Symbol: "ETHUSDT"},
			Perp: &TempInstrumentInfo{Symbol: "ETHUSDT"},
		}
	// qe.SubscribeIns["XRP"] = &Instrument[TempInstrumentInfo]{
	// 		Spot: &TempInstrumentInfo{Symbol: "XRPUSDT"},
	// 		Perp: &TempInstrumentInfo{Symbol: "XRPUSDT"},
	// 	}
	// qe.SubscribeIns["ADA"] = &Instrument[TempInstrumentInfo]{
	// 		Spot: &TempInstrumentInfo{Symbol: "ADAUSDT"},
	// 		Perp: &TempInstrumentInfo{Symbol: "ADAUSDT"},
	// 	}
	// qe.SubscribeIns["SOL"] = &Instrument[TempInstrumentInfo]{
	// 		Spot: &TempInstrumentInfo{Symbol: "SOLUSDT"},
	// 		Perp: &TempInstrumentInfo{Symbol: "SOLUSDT"},
	// 	}
	// qe.SubscribeIns["LINK"] = &Instrument[TempInstrumentInfo]{
	// 		Spot: &TempInstrumentInfo{Symbol: "LINKUSDT"},
	// 		Perp: &TempInstrumentInfo{Symbol: "LINKUSDT"},
	// 	}
	// subscribtions, err  := LoadSubscribes()
	// if err == nil {
	// 	if len(subscribtions) > 0 {
	// 		qe.Logger.Info("Load subscribes from file")
	// 		for _, v := range subscribtions {
	// 			sub := make([]string, 0)
	// 			for _, quote := range v {
	// 				// fmt.Printf("%+v\n", quote)
	// 				sub = append(sub, quote.Coin)
	// 			}
	// 			qe.Subscribe(sub)
	// 		}
	// 		return
	// 	}
	// }else{
	// 	qe.Logger.Warning(err)
	// }
	// insFisrt30 := qe.MatchFilter(qe.SubscribeIns)
	for k := range qe.SubscribeIns {
		qe.Subscribe([]string{k})
	}
}


// func (qe *BinanceQuoteEngine) MatchFilter(ins map[string]*Instrument[TempInstrumentInfo]) map[string]*Instrument[TempInstrumentInfo] {
// 	qe.Logger.Info("========= First 30 VOL coin =========")
// 	InsSlice := make([]*CpStruct[bybit_resp.InstrumentInfo], 0)
// 	for k, v := range ins {
// 		tickers, err := qe.Api.Market_Tickers("linear", bybit_http.WithQuery(map[string]string{"symbol": v.Perp.Symbol}))
// 		if err != nil {
// 			qe.Logger.Error(err)
// 		}

// 		if len(tickers) == 0 {
// 			delete(ins, k)
// 			continue
// 		}
// 		// fr, _ := strconv.ParseFloat(tickers[0].FR, 64)
// 		vol, _ := strconv.ParseFloat(tickers[0].Turnover24h, 64)
// 		InsSlice = append(InsSlice, &CpStruct[bybit_resp.InstrumentInfo]{
// 			Coin: k,
// 			Vol:  vol,
// 			Ins:  v,
// 		})
// 	}
// 	sort.Slice(InsSlice, func(i, j int) bool {
// 		return InsSlice[i].Vol > InsSlice[j].Vol
// 	})
// 	retIns := make(map[string]*Instrument[bybit_resp.InstrumentInfo])
// 	for i := 0; i < 30; i++ {
// 		retIns[InsSlice[i].Coin] = InsSlice[i].Ins
// 		qe.Logger.Infof("%s, Vol : %f", InsSlice[i].Coin, InsSlice[i].Vol)
// 	}
// 	qe.Logger.Infof("========= match =========")
// 	return retIns
// }

func (qe *BinanceQuoteEngine) GetSubscriptions() []string{
	return GetSubscriptions(qe.WsPool)
}