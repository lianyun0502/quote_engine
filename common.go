package quote_engine

import (
	"os"
	"os/signal"
	"syscall"
	"context"

	"github.com/sirupsen/logrus"
)

func WaitForClose(log *logrus.Logger, ctx context.Context) {
	sysSignalCh := make(chan os.Signal, 2)
	signal.Notify(sysSignalCh, os.Interrupt, syscall.SIGTERM)
	select {
	case sig := <-sysSignalCh:
		log.Info("receive signal: ", sig)
	case <-ctx.Done():
		log.Info("receive close signal")
	}
	log.Info("Closed")
}