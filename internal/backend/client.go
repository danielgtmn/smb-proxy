package backend

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/danielgietmann/smb-proxy/internal/config"
	"github.com/hirochachacha/go-smb2"
)

func Verify(ctx context.Context, cfg *config.Config) error {
	dialer := net.Dialer{Timeout: 15 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", cfg.RemoteAddress())
	if err != nil {
		return fmt.Errorf("connect to remote SMB server: %w", err)
	}
	defer conn.Close()

	smbDialer := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     cfg.RemoteUser,
			Password: cfg.RemotePassword,
			Domain:   cfg.RemoteDomain,
		},
	}

	session, err := smbDialer.DialContext(ctx, conn)
	if err != nil {
		return fmt.Errorf("authenticate with remote SMB server: %w", err)
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
