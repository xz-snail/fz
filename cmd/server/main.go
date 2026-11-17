package main

import (
	"log"

	"ctaf-fz/core"
	"ctaf-fz/global"
	"ctaf-fz/initialize"

	"go.uber.org/zap"
)

func main() {
	cfg, err := initialize.InitApp()
	if err != nil {
		log.Fatalf("initialize config: %v", err)
	}

	defer func() {
		_ = global.Logger.Sync()
	}()

	engine, app, err := initialize.InitServer(cfg)
	if err != nil {
		global.Logger.Fatal("initialize server failed", zap.Error(err))
	}

	if err := core.Run(engine, cfg, app); err != nil {
		global.Logger.Fatal("server stopped with error", zap.Error(err))
	}
}
