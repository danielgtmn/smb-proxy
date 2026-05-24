package config

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/danielgietmann/smb-proxy/internal/netutil"
)

const (
	ModeGateway = "gateway"
	ModeTCP     = "tcp"

	defaultMountOptions = "iocharset=utf8,rw,seal,vers=3.0,uid=0,gid=0,file_mode=0664,dir_mode=0775,noserverino"
)

type Config struct {
	Mode Mode

	RemoteHost     string
	RemoteHostIP   string
	RemotePort     int
	RemoteShare    string
	RemoteUser     string
	RemotePassword string
	RemoteDomain   string
	ForceIPv4      bool
	DialTimeout    time.Duration

	LocalShare     string
	LocalPort      int
	LocalUser      string
	LocalPassword  string
	AllowGuest     bool

	MountPath     string
	MountOptions  string
}

type Mode string

func Load() (*Config, error) {
	mode := Mode(strings.ToLower(strings.TrimSpace(envOr("SMB_PROXY_MODE", ModeGateway))))
	if mode != ModeGateway && mode != ModeTCP {
		return nil, fmt.Errorf("invalid SMB_PROXY_MODE %q: use %q or %q", mode, ModeGateway, ModeTCP)
	}

	remotePort, err := parsePort(envOr("SMB_PORT", "445"), "SMB_PORT")
	if err != nil {
		return nil, err
	}

	localPort, err := parsePort(envOr("LOCAL_PORT", "445"), "LOCAL_PORT")
	if err != nil {
		return nil, err
	}

	dialTimeout, err := parseDuration(envOr("SMB_DIAL_TIMEOUT", "30s"), "SMB_DIAL_TIMEOUT")
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Mode:           mode,
		RemoteHost:     strings.TrimSpace(os.Getenv("SMB_HOST")),
		RemoteHostIP:   strings.TrimSpace(os.Getenv("SMB_HOST_IP")),
		RemotePort:     remotePort,
		RemoteShare:    strings.TrimSpace(os.Getenv("SMB_SHARE")),
		RemoteUser:     strings.TrimSpace(os.Getenv("SMB_USER")),
		RemotePassword: os.Getenv("SMB_PASSWORD"),
		RemoteDomain:   strings.TrimSpace(os.Getenv("SMB_DOMAIN")),
		ForceIPv4:      parseBool(envOr("SMB_FORCE_IPV4", "true")),
		DialTimeout:    dialTimeout,
		LocalShare:     strings.TrimSpace(envOr("LOCAL_SHARE", "proxy")),
		LocalPort:      localPort,
		LocalUser:      strings.TrimSpace(envOr("LOCAL_USER", "proxy")),
		LocalPassword:  os.Getenv("LOCAL_PASSWORD"),
		AllowGuest:     parseBool(envOr("LOCAL_ALLOW_GUEST", "false")),
		MountPath:      strings.TrimSpace(envOr("MOUNT_PATH", "/mnt/remote")),
		MountOptions:   strings.TrimSpace(envOr("SMB_MOUNT_OPTIONS", defaultMountOptions)),
	}

	if cfg.RemoteHost == "" {
		return nil, fmt.Errorf("SMB_HOST is required")
	}

	if mode == ModeGateway {
		if cfg.RemoteShare == "" {
			return nil, fmt.Errorf("SMB_SHARE is required in gateway mode")
		}
		if cfg.RemoteUser == "" {
			return nil, fmt.Errorf("SMB_USER is required in gateway mode")
		}
		if cfg.RemotePassword == "" {
			return nil, fmt.Errorf("SMB_PASSWORD is required in gateway mode")
		}
		if cfg.LocalShare == "" {
			return nil, fmt.Errorf("LOCAL_SHARE must not be empty")
		}
	}

	return cfg, nil
}

func (c *Config) RemoteAddress() string {
	return fmt.Sprintf("%s:%d", c.RemoteHost, c.RemotePort)
}

func (c *Config) RemoteDialTarget() (address string, network string, err error) {
	if c.RemoteHostIP != "" {
		return fmt.Sprintf("%s:%d", c.RemoteHostIP, c.RemotePort), "tcp4", nil
	}

	if !c.ForceIPv4 {
		return c.RemoteAddress(), "tcp", nil
	}

	ip, err := netutil.ResolveIPv4(context.Background(), c.RemoteHost)
	if err != nil {
		return "", "", err
	}

	return fmt.Sprintf("%s:%d", ip, c.RemotePort), "tcp4", nil
}

func (c *Config) RemoteUNC() string {
	return fmt.Sprintf("//%s/%s", c.RemoteHost, c.RemoteShare)
}

func (c *Config) MountOptionString(credentialsPath string) (string, error) {
	options := c.MountOptions
	if credentialsPath != "" {
		options = fmt.Sprintf("credentials=%s,%s", credentialsPath, options)
	}

	if c.RemoteHostIP != "" {
		options = fmt.Sprintf("%s,ip=%s", options, c.RemoteHostIP)
		return options, nil
	}

	if c.ForceIPv4 {
		ip, err := netutil.ResolveIPv4(context.Background(), c.RemoteHost)
		if err != nil {
			return "", err
		}
		options = fmt.Sprintf("%s,ip=%s", options, ip)
	}

	return options, nil
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func parsePort(value, name string) (int, error) {
	port, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid port number", name)
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("%s must be between 1 and 65535", name)
	}
	return port, nil
}

func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parseDuration(value, name string) (time.Duration, error) {
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration (e.g. 30s)", name)
	}
	return d, nil
}
