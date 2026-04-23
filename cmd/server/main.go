package main

import (
	"log"

	"github.com/joho/godotenv"
	"github.com/luissebastian953/stratix-core/config"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("failed to load env: %v", err)
	}

	cfg := config.LoadConfig()

	app, err := NewApp(cfg)
	if err != nil {
		log.Fatalf("failed to start: %v", err)
	}

	go func() {
		if err := app.Start(cfg.Port); err != nil {
			log.Printf("server stopped: %v", err)
		}
	}()

	waitForShutdown()

	app.Shutdown()
}
