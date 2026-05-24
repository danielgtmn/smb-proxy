package proxy

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	"github.com/danielgietmann/smb-proxy/internal/config"
)

func Run(ctx context.Context, cfg *config.Config) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.LocalPort))
	if err != nil {
		return fmt.Errorf("listen on port %d: %w", cfg.LocalPort, err)
	}
	defer listener.Close()

	log.Printf("TCP proxy listening on :%d -> %s", cfg.LocalPort, cfg.RemoteAddress())

	var wg sync.WaitGroup
	defer wg.Wait()

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	for {
		clientConn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				return fmt.Errorf("accept connection: %w", err)
			}
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := handleConnection(ctx, cfg, clientConn); err != nil {
				log.Printf("proxy connection failed: %v", err)
			}
		}()
	}
}

func handleConnection(ctx context.Context, cfg *config.Config, clientConn net.Conn) error {
	defer clientConn.Close()

	dialer := net.Dialer{Timeout: cfg.DialTimeout}
	address, network, err := cfg.RemoteDialTarget()
	if err != nil {
		return fmt.Errorf("resolve remote target: %w", err)
	}

	remoteConn, err := dialer.DialContext(ctx, network, address)
	if err != nil {
		return fmt.Errorf("connect to remote %s: %w", address, err)
	}
	defer remoteConn.Close()

	errCh := make(chan error, 2)
	go pipe(errCh, clientConn, remoteConn)
	go pipe(errCh, remoteConn, clientConn)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func pipe(errCh chan<- error, dst net.Conn, src net.Conn) {
	_, err := io.Copy(dst, src)
	errCh <- err
}
