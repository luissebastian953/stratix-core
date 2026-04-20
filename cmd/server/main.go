package main

import (
	"log"

	"github.com/luissebastian953/stratix-core/config"
)

func main() {
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
