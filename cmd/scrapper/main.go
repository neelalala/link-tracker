package main

import (
	"context"
	"flag"
	"log"
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

	app, err := application.NewApp(ctx, *cfgPath, os.Stdout)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		if err := app.Start(ctx); err != nil {
			log.Printf("App stopped: %v", err)
			stop()
		}
	}()

	<-ctx.Done()

	app.Shutdown()
}
