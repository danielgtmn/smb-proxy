package backend

import (
	"context"
	"fmt"
	"net"

	"github.com/danielgietmann/smb-proxy/internal/config"
	"github.com/hirochachacha/go-smb2"
)

func Verify(ctx context.Context, cfg *config.Config) error {
	conn, err := Dial(ctx, cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	session, err := authenticate(ctx, conn, cfg)
	if err != nil {
		return err
	}
	defer session.Logoff()

	if cfg.Mode == config.ModeGateway {
		share, err := session.Mount(cfg.RemoteShare)
		if err != nil {
			return fmt.Errorf("mount remote share %q: %w", cfg.RemoteShare, err)
		}
		defer share.Umount()
	}

	return nil
}

func Dial(ctx context.Context, cfg *config.Config) (net.Conn, error) {
	dialer := net.Dialer{Timeout: cfg.DialTimeout}
	address, network, err := cfg.RemoteDialTarget()
	if err != nil {
		return nil, fmt.Errorf("resolve remote SMB server: %w", err)
	}

	conn, err := dialer.DialContext(ctx, network, address)
	if err != nil {
		return nil, fmt.Errorf("connect to remote SMB server at %s: %w", address, err)
	}

	return conn, nil
}

func authenticate(ctx context.Context, conn net.Conn, cfg *config.Config) (*smb2.Session, error) {
	smbDialer := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     cfg.RemoteUser,
			Password: cfg.RemotePassword,
			Domain:   cfg.RemoteDomain,
		},
	}

	session, err := smbDialer.DialContext(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("authenticate with remote SMB server: %w", err)
	}

	return session, nil
}
