package main

import (
	"bufio"
	"encoding/json"
	"sort"
	"strconv"

	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-gota/gota/dataframe"

	// "github.com/go-gota/gota/series"
	// "github.com/go-gota/gota/series"
	// "gonum.org/v1/gonum/stat"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
	// "github.com/go-gota/gota/dataframe"
	"github.com/gocarina/gocsv"
	// "github.com/lianyun0502/crypto_strategy/funding_rate_strategy"
)

type Data struct {
	Time        int64             `json:"T"`
	Symbol      string            `json:"S"`
	Bids        map[string]string `json:"Bids"`
	Asks        map[string]string `json:"Asks"`
	BestBid     float64
	BestAsk     float64
	BestBidSize float64
	BestAskSize float64
}
type CSVData struct {
	Time        int64
	Symbol      string
	BestBid     float64
	BestAsk     float64
	BestBidSize float64
	BestAskSize float64
}

var Symbol = "TIAUSDT"

func main() {
	// coin := "TIA"
	D := make([]CSVData, 0)

	filePath := "../build/multi_bybit_spot/data/orderbook_202410230700.data"
	f, err := os.OpenFile(filePath, os.O_RDONLY, 0644)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	r := bufio.NewReader(f)
	for i := 0; ; i++ {
		line, _, err := r.ReadLine()
		if err != nil {
			break
		}
		var d Data
		json.Unmarshal(line, &d)
		if d.Symbol != Symbol {
			continue
		}
		if d.Bids == nil || d.Asks == nil {
			fmt.Println(string(line))
			continue
		}
		fmt.Println(i)
		dd := CSVData{
			Time:   d.Time,
			Symbol: d.Symbol,
		}
		dd.BestBid, dd.BestBidSize = GetBestBid(d.Bids)
		dd.BestAsk, dd.BestAskSize = GetBestAsk(d.Asks)
		D = append(D, dd)
	}

	df := dataframe.LoadStructs(D)
	fmt.Println(df.Describe())
	// fmt.Println(df)
	// df2 := SetMeanSd(&df, 600)

	file := filepath.Base(filePath)
	SaveCSV(D, strings.ReplaceAll(file, "data", "csv"))

	// fmt.Println(df2.Describe())
	p := plot.New()
	// p.Title.Text = "Spread Ratio"
	// p.X.Label.Text = "Time"
	// p.Y.Label.Text = "Spread"

	plotutil.AddLinePoints(
		p,
		"BestBidSize", MakePoint(df.Col("BestBidSize").Float()),
	)
	// plotutil.AddLines(
	// 	p,
	// 	"PSMean+2sd", MakePoint(df2.Col("PS2sd").Float()),
	// 	"SPMean+2sd", MakePoint(df2.Col("SP2sd").Float()),
	// 	"PSMean+3sd", MakePoint(df2.Col("PS3sd").Float()),
	// 	"SPMean+3sd", MakePoint(df2.Col("SP3sd").Float()),
	// )

	file = strings.ReplaceAll(file, "data", "png")

	if err := p.Save(20*vg.Inch, 8*vg.Inch, file); err != nil {
		panic(err)
	}
}

func GetBestBid(orders map[string]string) (float64, float64) {
	bids := make([]string, 0)
	for price, _ := range orders {
		bids = append(bids, price)
	}
	sort.Slice(bids, func(i, j int) bool {
		return bids[i] > bids[j]
	})
	bestBids, _ := strconv.ParseFloat(bids[0], 64)
	bestBidSize, _ := strconv.ParseFloat(orders[bids[0]], 64)
	return bestBids, bestBidSize
}

func GetBestAsk(orders map[string]string) (float64, float64) {
	asks := make([]string, 0)
	for price, _ := range orders {
		asks = append(asks, price)
	}
	sort.Slice(asks, func(i, j int) bool {
		return asks[i] < asks[j]
	})
	bestAsk, _ := strconv.ParseFloat(asks[0], 64)
	bestAskSize, _ := strconv.ParseFloat(orders[asks[0]], 64)
	return bestAsk, bestAskSize
}

func MakePoint[t float32 | float64 | int](data []t) plotter.XYs {
	pts := make(plotter.XYs, len(data))
	for i, d := range data {
		pts[i].X = float64(i)
		pts[i].Y = float64(d)
	}
	return pts
}

// func SetMeanSd(df *dataframe.DataFrame, window int) (*dataframe.DataFrame) {
// 	SPMean := make([]float64, df.Nrow())
// 	PSMean := make([]float64, df.Nrow())
// 	SP2sd := make([]float64, df.Nrow())
// 	PS2sd := make([]float64, df.Nrow())
// 	SP3sd := make([]float64, df.Nrow())
// 	PS3sd := make([]float64, df.Nrow())

// 	for i := 0; i < df.Nrow(); i++ {
// 		fmt.Println(i)
// 		if i < window {
// 			SPMean[i] = 0
// 			PSMean[i] = 0
// 			SP2sd[i] = 0
// 			PS2sd[i] = 0
// 			SP3sd[i] = 0
// 			PS3sd[i] = 0
// 			continue
// 		}
// 		m, sd := stat.MeanStdDev(df.Col("SPSpread").Float()[i-window:i], nil)
// 		SPMean[i] = m
// 		SP2sd[i] = m+2.5*sd
// 		SP3sd[i] = m+3*sd
// 		m, sd = stat.MeanStdDev(df.Col("PSSpread").Float()[i-window:i], nil)
// 		PSMean[i] = m
// 		PS2sd[i] = m+2.5*sd
// 		PS3sd[i] = m+3*sd
// 	}
// 	df2 := dataframe.New(
// 		series.New(SPMean, series.Float, "SPMean"),
// 		series.New(PSMean, series.Float, "PSMean"),
// 		series.New(SP2sd, series.Float, "SP2sd"),
// 		series.New(PS2sd, series.Float, "PS2sd"),
// 		series.New(SP3sd, series.Float, "SP3sd"),
// 		series.New(PS3sd, series.Float, "PS3sd"),
// 	)
// 	ret := df.CBind(df2)
// 	return &ret
// }

func SaveCSV[T any](data []T, fileName string) {
	f, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	// df.WriteCSV(f, dataframe.WriteHeader(true))
	gocsv.MarshalFile(data, f)

}
