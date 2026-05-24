package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/danielgietmann/smb-proxy/internal/backend"
	"github.com/danielgietmann/smb-proxy/internal/config"
	"github.com/danielgietmann/smb-proxy/internal/gateway"
	"github.com/danielgietmann/smb-proxy/internal/proxy"
)

var version = "dev"

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)
	log.Printf("smb-proxy %s", version)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if cfg.Mode == config.ModeGateway {
		log.Printf("starting gateway mode for %s", cfg.RemoteUNC())
		if address, _, err := cfg.RemoteDialTarget(); err == nil {
			log.Printf("remote SMB dial target: %s", address)
		}
		if err := backend.Verify(ctx, cfg); err != nil {
			log.Fatalf("remote SMB verification failed: %v", err)
		}
		if err := gateway.Run(cfg); err != nil {
			log.Fatalf("gateway failed: %v", err)
		}
		return
	}

	log.Printf("starting TCP proxy mode to %s", cfg.RemoteAddress())
	if err := proxy.Run(ctx, cfg); err != nil && err != context.Canceled {
		log.Fatalf("proxy failed: %v", err)
	}
	os.Exit(0)
}
