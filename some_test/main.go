package main

import (
	"time"
	"context"

	nested "github.com/antonfisher/nested-logrus-formatter"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/lianyun0502/quote_engine/engine"
	"github.com/rifflock/lfshook"
	"github.com/sirupsen/logrus"
)


func main() {
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)
	log.SetFormatter(&nested.Formatter{
		HideKeys:    true,
		TimestampFormat: "2006-01-02 15:04:05.000000",
		FieldsOrder: []string{"component", "category"},
	  })
	writer, _ := rotatelogs.New(
		"Log" + "_%Y%m%d%H%M.log",
		rotatelogs.WithMaxAge(time.Duration(4)*24*time.Hour),
		rotatelogs.WithRotationTime(time.Duration(1)*time.Hour),
	)
	writerMap := lfshook.WriterMap{
		logrus.InfoLevel:  writer,
		logrus.ErrorLevel: writer,
		logrus.DebugLevel: writer,
		logrus.FatalLevel: writer,
		logrus.WarnLevel:  writer,
		logrus.PanicLevel: writer,
	}
	hook := lfshook.NewHook(writerMap, &logrus.JSONFormatter{TimestampFormat: "2006-01-02 15:04:05.000000"})
	log.AddHook(hook)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Second)
		cancel()
	}()
	log.WithField("file", "main").WithField("category", "test").Info("wait for close")
	quote_engine.WaitForClose(log, ctx)
	
}