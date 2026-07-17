package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/gabrielctavares/cs2sj-highlights/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	app := cli.DefaultApp()
	os.Exit(app.Run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
