package datastorage

import (
	"context"
	// "fmt"
	"path/filepath"
	"time"

	"github.com/lianyun0502/quote_engine/configs"
	rotatefile "github.com/lianyun0502/quote_engine/rotate_file"
	"github.com/sirupsen/logrus"
)

func WithOrderbookTxtHandle(ctx context.Context, log *logrus.Logger, cfg *configs.DataConfig) func([]byte) {
	log.Info("create orderbook file")
	file_path := filepath.Join(cfg.Dir, "orderbook", "%Y%m%d%H%M.data")
	writer, err := rotatefile.New(
		file_path,
		rotatefile.WithMaxAge(time.Duration(cfg.MaxAge)*time.Hour),
		rotatefile.WithRotationTime(time.Duration(cfg.RotationTime)*time.Hour),
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

func WithTickerTxtHandle(ctx context.Context, log *logrus.Logger, cfg *configs.DataConfig) func([]byte) {
	log.Info("create ticker file")
	file_path := filepath.Join(cfg.Dir, "market", "%Y%m%d%H%M.data")
	writer, err := rotatefile.New(
		file_path,
		rotatefile.WithMaxAge(time.Duration(cfg.MaxAge)*time.Hour),
		rotatefile.WithRotationTime(time.Duration(cfg.RotationTime)*time.Hour),
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


func WithTradeTxtHandle(ctx context.Context, log *logrus.Logger, cfg *configs.DataConfig) func([]byte) {
	log.Info("create trade file")
	file_path := filepath.Join(cfg.Dir, "trade", "%Y%m%d%H%M.data")
	writer, err := rotatefile.New(
		file_path,
		rotatefile.WithMaxAge(time.Duration(cfg.MaxAge)*time.Hour),
		rotatefile.WithRotationTime(time.Duration(cfg.RotationTime)*time.Hour),
	)
	if err != nil {
		writer.Close()
		panic(err)
	}
	go func() {
		<-ctx.Done()
		log.Info("close trade file")
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