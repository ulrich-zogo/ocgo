package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	AppName     = "ocgo"
	DefaultHost = "127.0.0.1"
	DefaultPort = 3456
	OpenAIURL   = "https://opencode.ai/zen/go/v1/chat/completions"
)

type Config struct {
	APIKey string `json:"api_key"`
	Host   string `json:"host"`
	Port   int    `json:"port"`
}

func ConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", AppName)
}

func ConfigFile() string {
	return filepath.Join(ConfigDir(), "config.json")
}

func PIDFile() string {
	return filepath.Join(ConfigDir(), "ocgo.pid")
}

func LogFile() string {
	return filepath.Join(ConfigDir(), "ocgo.log")
}

var ModelMappingFile = func() string {
	return filepath.Join(ConfigDir(), "model-mapping.json")
}

func SaveConfig(cfg Config) error {
	if err := os.MkdirAll(ConfigDir(), 0755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(ConfigFile(), append(b, '\n'), 0600); err != nil {
		return err
	}
	fmt.Printf("Saved config to %s\n", ConfigFile())
	return nil
}

func LoadConfig() (Config, error) {
	cfg := Config{Host: DefaultHost, Port: DefaultPort, APIKey: os.Getenv("OCGO_API_KEY")}
	b, err := os.ReadFile(ConfigFile())
	if err == nil {
		_ = json.Unmarshal(b, &cfg)
	}
	if cfg.APIKey == "" {
		return cfg, errors.New("missing API key; run: ocgo setup")
	}
	if cfg.Host == "" {
		cfg.Host = DefaultHost
	}
	if cfg.Port == 0 {
		cfg.Port = DefaultPort
	}
	return cfg, nil
}

func ReadPID() (int, error) {
	b, err := os.ReadFile(PIDFile())
	if err != nil {
		return 0, err
	}
	var pid int
	_, err = fmt.Sscan(strings.TrimSpace(string(b)), &pid)
	return pid, err
}
