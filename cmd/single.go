// Copyright (c) Elliot Peele <elliot@bentlogic.net>

package cmd

import (
	"context"
	"os"
	"os/signal"
)

func setupSignalHandler(ctx context.Context, cancel context.CancelFunc) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	select {
	case s := <-ch:
		logger.Infof("Received signal %s; aborting", s)
		cancel()
	case <-ctx.Done():
	}
	signal.Stop(ch)
}
