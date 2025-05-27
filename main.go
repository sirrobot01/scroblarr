package main

import (
	"context"
	"flag"
	"github.com/sirrobot01/scroblarr/cmd/scroblarr"
	"github.com/sirrobot01/scroblarr/internal/config"
	"log"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "/data", "path to the data folder")
	flag.Parse()

	config.SetConfigPath(configPath)
	config.Get() // This will initialize the config
	ctx := context.Background()
	if err := scroblarr.Start(ctx); err != nil {
		log.Fatal(err)
	}
}
