package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/application"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfgPath := flag.String("config", "scrapper.conf", "path to config file")
	flag.Parse()

	app, cleanup := application.NewApp(ctx, *cfgPath, os.Stdout)
	defer cleanup()
	app.Start(ctx)
}
