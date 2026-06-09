package process

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"

	"ocgo/internal/config"
)

func StartBackground() error {
	_, err := StartServerProcess(true)
	return err
}

func StartServerProcess(detached bool) (*exec.Cmd, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable: %w", err)
	}

	cmd := exec.Command(exe, "serve")

	if err := os.MkdirAll(config.ConfigDir(), 0755); err != nil {
		return nil, fmt.Errorf("failed to create config dir: %w", err)
	}

	logFile := config.LogFile()
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	cmd.Stdout = f
	cmd.Stderr = f

	if detached {
		cmd.SysProcAttr = DetachedAttrs()
	}

	if err := cmd.Start(); err != nil {
		f.Close()
		return nil, fmt.Errorf("failed to start server: %w", err)
	}

	if detached {
		go func() {
			cmd.Wait()
			f.Close()
		}()
	}

	return cmd, nil
}

func StartLaunchServer(base string) (*exec.Cmd, error) {
	if Healthy(base) {
		return nil, nil
	}

	cmd, err := StartServerProcess(false)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			cmd.Process.Kill()
			cmd.Wait()
			return nil, errors.New("server failed to start within 10 seconds")
		default:
			if Healthy(base) {
				return cmd, nil
			}
			time.Sleep(200 * time.Millisecond)
		}
	}
}

func StopManagedServer(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		cmd.Process.Kill()
		cmd.Wait()
	}
	os.Remove(config.PIDFile())
}

func Healthy(base string) bool {
	c := http.Client{Timeout: 500 * time.Millisecond}
	resp, err := c.Get(base + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return resp.StatusCode == http.StatusOK
}

func EnsureServer(base string) error {
	if Healthy(base) {
		return nil
	}

	if err := StartBackground(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return errors.New("server failed to start within 10 seconds")
		default:
			if Healthy(base) {
				return nil
			}
			time.Sleep(200 * time.Millisecond)
		}
	}
}

func BaseURL(cfg config.Config) string {
	host := cfg.Host
	if host == "" {
		host = config.DefaultHost
	}
	port := cfg.Port
	if port == 0 {
		port = config.DefaultPort
	}
	return fmt.Sprintf("http://%s:%d", host, port)
}

func WaitHealthy(base string, timeout time.Duration) error {
	if Healthy(base) {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("server did not become healthy at %s within %s", base, timeout)
		default:
			if Healthy(base) {
				return nil
			}
			time.Sleep(200 * time.Millisecond)
		}
	}
}

func StartBackgroundWithWait(base string, timeout time.Duration) (*exec.Cmd, error) {
	if Healthy(base) {
		return nil, nil
	}
	cmd, err := StartServerProcess(true)
	if err != nil {
		return nil, err
	}
	if err := WaitHealthy(base, timeout); err != nil {
		return cmd, err
	}
	return cmd, nil
}

func KillPID(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid: %d", pid)
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Kill()
}
