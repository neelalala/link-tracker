package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfgPath := flag.String("config", "bot.conf", "path to config file")
	flag.Parse()

	app := application.NewApp(*cfgPath, os.Stdout)
	app.Start(ctx)
}
