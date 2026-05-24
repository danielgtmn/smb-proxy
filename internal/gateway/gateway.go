package gateway

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/danielgietmann/smb-proxy/internal/config"
)

func Run(cfg *config.Config) error {
	if err := os.MkdirAll(cfg.MountPath, 0o755); err != nil {
		return fmt.Errorf("create mount path: %w", err)
	}

	if err := mountRemote(cfg); err != nil {
		return err
	}
	defer unmount(cfg.MountPath)

	if err := writeSambaConfig(cfg); err != nil {
		return err
	}

	log.Printf("exporting %s as \\\\localhost\\%s on port %d", cfg.RemoteUNC(), cfg.LocalShare, cfg.LocalPort)
	return execSamba(cfg)
}

func mountRemote(cfg *config.Config) error {
	if isMounted(cfg.MountPath) {
		log.Printf("mount path %s already mounted", cfg.MountPath)
		return nil
	}

	credentialsPath := "/run/smb-proxy/credentials"
	if err := os.MkdirAll(filepath.Dir(credentialsPath), 0o700); err != nil {
		return fmt.Errorf("create credentials directory: %w", err)
	}

	credentials := fmt.Sprintf("username=%s\npassword=%s\n", cfg.RemoteUser, cfg.RemotePassword)
	if cfg.RemoteDomain != "" {
		credentials += fmt.Sprintf("domain=%s\n", cfg.RemoteDomain)
	}

	if err := os.WriteFile(credentialsPath, []byte(credentials), 0o600); err != nil {
		return fmt.Errorf("write credentials file: %w", err)
	}

	mountOptions, err := cfg.MountOptionString(credentialsPath)
	if err != nil {
		return fmt.Errorf("build mount options: %w", err)
	}

	options := []string{
		"-t", "cifs",
		cfg.RemoteUNC(),
		cfg.MountPath,
		"-o", mountOptions,
	}

	log.Printf("mounting remote share %s", cfg.RemoteUNC())
	cmd := exec.Command("mount", options...)
	var stderr bytes.Buffer
	cmd.Stdout = os.Stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return fmt.Errorf("mount remote share: %w: %s", err, msg)
		}
		return fmt.Errorf("mount remote share: %w", err)
	}

	return nil
}

func unmount(mountPath string) {
	if !isMounted(mountPath) {
		return
	}

	cmd := exec.Command("umount", mountPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Printf("warning: failed to unmount %s: %v", mountPath, err)
	}
}

func isMounted(path string) bool {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return false
	}
	prefix := path + " "
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == path {
			return true
		}
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

func writeSambaConfig(cfg *config.Config) error {
	if err := os.MkdirAll("/etc/samba", 0o755); err != nil {
		return fmt.Errorf("create samba config directory: %w", err)
	}

	var shareBlock strings.Builder
	fmt.Fprintf(&shareBlock, "\n[%s]\n", cfg.LocalShare)
	fmt.Fprintf(&shareBlock, "    path = %s\n", cfg.MountPath)
	fmt.Fprintf(&shareBlock, "    browseable = yes\n")
	fmt.Fprintf(&shareBlock, "    read only = no\n")
	fmt.Fprintf(&shareBlock, "    create mask = 0664\n")
	fmt.Fprintf(&shareBlock, "    directory mask = 0775\n")

	if cfg.AllowGuest {
		fmt.Fprintf(&shareBlock, "    guest ok = yes\n")
		fmt.Fprintf(&shareBlock, "    map to guest = Bad User\n")
	} else {
		fmt.Fprintf(&shareBlock, "    guest ok = no\n")
		fmt.Fprintf(&shareBlock, "    valid users = %s\n", cfg.LocalUser)
	}

	configBody := fmt.Sprintf(`[global]
    workgroup = WORKGROUP
    server string = smb-proxy
    security = user
    map to guest = Bad User
    load printers = no
    printing = bsd
    disable spoolss = yes
    smb ports = %d
    pid directory = /run/smb-proxy
    lock directory = /run/smb-proxy
    state directory = /run/smb-proxy
    cache directory = /run/smb-proxy
    private dir = /run/smb-proxy/private
    passdb backend = tdbsam
%s
`, cfg.LocalPort, shareBlock.String())

	if err := os.MkdirAll("/run/smb-proxy/private", 0o700); err != nil {
		return fmt.Errorf("create samba runtime directory: %w", err)
	}

	if err := os.WriteFile("/etc/samba/smb.conf", []byte(configBody), 0o644); err != nil {
		return fmt.Errorf("write smb.conf: %w", err)
	}

	if cfg.AllowGuest {
		return nil
	}

	if cfg.LocalPassword == "" {
		return fmt.Errorf("LOCAL_PASSWORD is required when LOCAL_ALLOW_GUEST=false")
	}

	cmd := exec.Command("smbpasswd", "-a", "-s", cfg.LocalUser)
	cmd.Stdin = strings.NewReader(cfg.LocalPassword + "\n" + cfg.LocalPassword + "\n")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("create local samba user: %w", err)
	}

	enableCmd := exec.Command("smbpasswd", "-e", cfg.LocalUser)
	enableCmd.Stdout = os.Stdout
	enableCmd.Stderr = os.Stderr
	if err := enableCmd.Run(); err != nil {
		return fmt.Errorf("enable local samba user: %w", err)
	}

	return nil
}

func execSamba(cfg *config.Config) error {
	cmd := exec.Command(
		"smbd",
		"--foreground",
		"--no-process-group",
		"--debug-stdout",
		"-s", "/etc/samba/smb.conf",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run smbd: %w", err)
	}

	return nil
}
