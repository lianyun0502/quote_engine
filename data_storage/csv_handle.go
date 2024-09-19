package datastorage

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"strconv"
	"time"

	"github.com/lianyun0502/exchange_conn/v1"
	"github.com/sirupsen/logrus"
)

func WithOrderbookCsvHandle(log *logrus.Logger) func([]byte) {
	csvFile, err := os.Create("orderbook.csv")
	if err != nil {
		csvFile.Close()
		panic(err)
	}
	writer := csv.NewWriter(csvFile)
	writer.Write([]string{"topic", "localtime", "symbol", "time", "ask1", "vol1", "bid1", "vol1"})
	return func(rawData []byte) {
		ob := new(exchange_conn.OrderBookStream)
		json.Unmarshal(rawData, ob)
		data := []string{
			ob.Topic,
			strconv.FormatInt(time.Now().UnixNano(), 10),
			ob.Symbol,
			strconv.FormatInt(ob.Time, 10),
		}
		for ask, vol := range ob.Asks {
			data = append(data, ask, vol)
		}
		for bid, vol := range ob.Bids {
			data = append(data, bid, vol)
		}
		err := writer.Write(data)
		if err != nil {
			log.Error(err)
		}
	}

}

func WithTickerCsvHandle(log *logrus.Logger) func([]byte) {
	csvFile, err := os.Create("ticker.csv")
	if err != nil {
		csvFile.Close()
		panic(err)
	}
	writer := csv.NewWriter(csvFile)
	writer.Write([]string{"topic", "localtime", "symbol", "time", "fundingRate", "nextFundingTime", "indexPrice", "marketPrice"})
	return func(rawData []byte) {
		ticker := new(exchange_conn.MarKetPriceStream)
		json.Unmarshal(rawData, ticker)
		data := []string{
			ticker.Topic,
			strconv.FormatInt(time.Now().UnixNano(), 10),
			ticker.Symbol,
			strconv.FormatInt(ticker.Time, 10),
			ticker.FundingRate,
			strconv.FormatInt(ticker.NextFundingTime, 10),
			ticker.IndexPrice,
			ticker.MarketPrice,
		}
		err := writer.Write(data)
		if err != nil {
			log.Error(err)
		}
	}

}
