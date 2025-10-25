package main

import (
	"fmt"
	"log"

	"github.com/matt-host/blog-agg/internal/config"
)

func main() {
	cfg, err := config.Read()
	if err != nil {
		log.Fatalf("Failed to read config file: %v\n", err)
	}

	err = cfg.SetUser("lane")
	if err != nil {
		log.Fatalf("Failed to set the user: %v\n", err)
	}

	cfg, err = config.Read()
	if err != nil {
		log.Fatalf("Failed to read config file: %v\n", err)
	}

	fmt.Println(cfg)
}
