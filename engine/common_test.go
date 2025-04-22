package quote_engine_test

import (
	"context"
	"testing"
	"time"

	"github.com/lianyun0502/quote_engine/engine"
	"github.com/sirupsen/logrus"
)



func TestWaitForClose(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Second)
		cancel()
	}()
	quote_engine.WaitForClose(log, ctx)
}