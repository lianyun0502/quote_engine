package datastorage

import (
	"context"
	rotatefile "github.com/lianyun0502/quote_engine/rotate_file"
	"github.com/sirupsen/logrus"
	"time"
)

func WithOrderbookTxtHandle(ctx context.Context, log *logrus.Logger) func([]byte) {
	log.Info("create orderbook file")
	writer, err := rotatefile.New(
		"data/orderbook_%Y%m%d%H%M.data",
		rotatefile.WithMaxAge(time.Duration(24*7)*time.Hour),
		rotatefile.WithRotationTime(time.Duration(1)*time.Hour),
	)
	if err != nil {
		writer.Close()
		panic(err)
	}
	go func() {
		<-ctx.Done()
		log.Info("close orderbook file")
		writer.Close()
	}()
	return func(rawData []byte) {
		_, err := writer.Write(rawData)
		if err != nil {
			log.Error(err)
		}
		writer.Write([]byte("\n"))
	}

}

func WithTickerTxtHandle(ctx context.Context, log *logrus.Logger) func([]byte) {
	log.Info("create ticker file")
	writer, err := rotatefile.New(
		"data/ticker_%Y%m%d%H%M.data",
		rotatefile.WithMaxAge(time.Duration(24*7)*time.Hour),
		rotatefile.WithRotationTime(time.Duration(1)*time.Hour),
	)
	if err != nil {
		writer.Close()
		panic(err)
	}
	go func() {
		<-ctx.Done()
		log.Info("close ticker file")
		writer.Close()
	}()
	return func(rawData []byte) {
		_, err := writer.Write(rawData)
		if err != nil {
			log.Error(err)
		}
		writer.Write([]byte("\n"))
	}

}
