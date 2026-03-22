//go:build linux

package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Register sets up thawts to start on login.
// Prefers systemd user service; falls back to XDG autostart for non-systemd desktops.
func Register(execPath string) error {
	if systemdAvailable() {
		return registerSystemd(execPath)
	}
	return registerXDG(execPath)
}

func Unregister() error {
	err1 := unregisterSystemd()
	err2 := unregisterXDG()
	if err1 != nil {
		return err1
	}
	return err2
}

func IsRegistered() bool {
	return isSystemdRegistered() || isXDGRegistered()
}

// ── systemd ───────────────────────────────────────────────────────────────────

func systemdAvailable() bool {
	_, err := exec.LookPath("systemctl")
	return err == nil
}

func systemdServicePath() (string, error) {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfg, "systemd", "user", "thawts.service"), nil
}

func serviceContent(execPath string) string {
	return fmt.Sprintf("[Unit]\nDescription=Thawts thought capture\n\n[Service]\nExecStart=%s\nRestart=on-failure\n\n[Install]\nWantedBy=default.target\n", execPath)
}

func registerSystemd(execPath string) error {
	path, err := systemdServicePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(serviceContent(execPath)), 0o644); err != nil {
		return err
	}
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	if err := exec.Command("systemctl", "--user", "enable", "--now", "thawts").Run(); err != nil {
		return fmt.Errorf("systemctl enable: %w", err)
	}
	return nil
}

func unregisterSystemd() error {
	_ = exec.Command("systemctl", "--user", "disable", "--now", "thawts").Run()
	path, err := systemdServicePath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	return nil
}

func isSystemdRegistered() bool {
	path, err := systemdServicePath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// ── XDG autostart (non-systemd desktops) ─────────────────────────────────────

func xdgDesktopPath() (string, error) {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfg, "autostart", "thawts.desktop"), nil
}

func desktopContent(execPath string) string {
	return fmt.Sprintf("[Desktop Entry]\nType=Application\nName=Thawts\nExec=%s\nHidden=false\nX-GNOME-Autostart-enabled=true\n", execPath)
}

func registerXDG(execPath string) error {
	path, err := xdgDesktopPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(desktopContent(execPath)), 0o644)
}

func unregisterXDG() error {
	path, err := xdgDesktopPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func isXDGRegistered() bool {
	path, err := xdgDesktopPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}
