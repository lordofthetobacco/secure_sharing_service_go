package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"bx.share/config"
	"bx.share/internal/api"
	"bx.share/internal/store"

	"github.com/redis/go-redis/v9"
)

func main() {
	configPath := flag.String("config", "", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal("config error:", err)
	}

	st := initStore(cfg)
	defer st.Close()

	router := api.SetupRouter(st, cfg)

	log.Printf("Server starting on %s", cfg.Addr())
	log.Printf("Base URL: %s", cfg.Server.BaseURL)
	log.Printf("Store: %s", cfg.Store.Type)

	server := &http.Server{
		Addr:         cfg.Addr(),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Fatal(server.ListenAndServe())
}

func initStore(cfg *config.Config) store.Store {
	switch cfg.Store.Type {
	case "redis":
		st, err := store.NewRedisStore(&redis.Options{
			Addr:     cfg.Store.Redis.Addr,
			Password: cfg.Store.Redis.Password,
			DB:       cfg.Store.Redis.DB,
		})
		if err != nil {
			log.Fatal("redis connection failed:", err)
		}
		return st
	default:
		return store.NewMemoryStore(30 * time.Second)
	}
}
