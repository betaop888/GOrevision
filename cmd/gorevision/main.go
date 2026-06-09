package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gorevision/internal/config"
	"gorevision/internal/lb"
	"gorevision/internal/proxy"
)

func main() {
	configPath := flag.String("config", "config.json", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	backends, err := lb.NewBackends(cfg.Backends)
	if err != nil {
		log.Fatalf("create backends: %v", err)
	}

	balancer, err := lb.NewBalancer(cfg.Algorithm, backends)
	if err != nil {
		log.Fatalf("create balancer: %v", err)
	}

	checker := lb.NewHealthChecker(backends, cfg.HealthCheck)
	checker.Start()
	defer checker.Stop()

	handler := proxy.NewHandler(balancer)
	server := &http.Server{
		Addr:              cfg.Listen,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("GOrevision listening on %s with algorithm=%s", cfg.Listen, cfg.Algorithm)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Println("shutting down")
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}
