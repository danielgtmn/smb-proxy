package config

import (
	"strings"
	"testing"
)

func TestLoadGatewayDefaults(t *testing.T) {
	t.Setenv("SMB_PROXY_MODE", "gateway")
	t.Setenv("SMB_HOST", "nas.example.com")
	t.Setenv("SMB_SHARE", "data")
	t.Setenv("SMB_USER", "backup")
	t.Setenv("SMB_PASSWORD", "secret")
	t.Setenv("LOCAL_PASSWORD", "local")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Mode != ModeGateway {
		t.Fatalf("Mode = %q, want %q", cfg.Mode, ModeGateway)
	}
	if cfg.RemotePort != 445 {
		t.Fatalf("RemotePort = %d, want 445", cfg.RemotePort)
	}
	if cfg.LocalShare != "proxy" {
		t.Fatalf("LocalShare = %q, want proxy", cfg.LocalShare)
	}
	if !cfg.ForceIPv4 {
		t.Fatalf("ForceIPv4 = false, want true by default")
	}
	if cfg.RemoteUNC() != "//nas.example.com/data" {
		t.Fatalf("RemoteUNC() = %q", cfg.RemoteUNC())
	}
}

func TestMountOptionStringUsesHostIP(t *testing.T) {
	cfg := &Config{
		RemoteHost:   "u599718.your-storagebox.de",
		RemoteHostIP: "91.98.255.149",
		MountOptions: defaultMountOptions,
	}

	opts, err := cfg.MountOptionString("/run/smb-proxy/credentials")
	if err != nil {
		t.Fatalf("MountOptionString() error = %v", err)
	}
	if !strings.Contains(opts, "ip=91.98.255.149") {
		t.Fatalf("MountOptionString() = %q, want ip=91.98.255.149", opts)
	}
	if !strings.Contains(opts, "seal") {
		t.Fatalf("MountOptionString() = %q, want seal option", opts)
	}
}

func TestLoadTCPMode(t *testing.T) {
	t.Setenv("SMB_PROXY_MODE", "tcp")
	t.Setenv("SMB_HOST", "10.0.0.5")
	t.Setenv("LOCAL_PORT", "1445")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Mode != ModeTCP {
		t.Fatalf("Mode = %q, want %q", cfg.Mode, ModeTCP)
	}
	if cfg.LocalPort != 1445 {
		t.Fatalf("LocalPort = %d, want 1445", cfg.LocalPort)
	}
}

func TestLoadRequiresHost(t *testing.T) {
	t.Setenv("SMB_PROXY_MODE", "tcp")
	t.Setenv("SMB_HOST", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() expected error for missing SMB_HOST")
	}
}

func TestLoadGatewayRequiresCredentials(t *testing.T) {
	t.Setenv("SMB_PROXY_MODE", "gateway")
	t.Setenv("SMB_HOST", "nas.example.com")
	t.Setenv("SMB_SHARE", "data")
	t.Setenv("SMB_USER", "backup")
	t.Setenv("SMB_PASSWORD", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() expected error for missing SMB_PASSWORD")
	}
}
