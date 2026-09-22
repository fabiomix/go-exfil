package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type SFTPConfig struct {
	Host           string `yaml:"host"`
	Port           int    `yaml:"port"`
	User           string `yaml:"user"`
	Password       string `yaml:"password"`
	KeyFile        string `yaml:"key_file"`
	OnlyKnownHosts bool   `yaml:"only_known_hosts"`
}

type FileEntry struct {
	Local  string `yaml:"local"`
	Remote string `yaml:"remote"`
}

type AppConfig struct {
	SFTP        SFTPConfig  `yaml:"sftp"`
	ExitOnError bool        `yaml:"exit_on_error"`
	Overwrite   bool        `yaml:"overwrite"`
	Files       []FileEntry `yaml:"files"`
}

func (cfg *AppConfig) validate() error {
	if cfg.SFTP.Host == "" {
		return fmt.Errorf("sftp.host is required")
	}
	if cfg.SFTP.Port == 0 {
		return fmt.Errorf("sftp.port is required")
	}
	if cfg.SFTP.User == "" {
		return fmt.Errorf("sftp.user is required")
	}
	if cfg.SFTP.Password == "" && cfg.SFTP.KeyFile == "" {
		return fmt.Errorf("either sftp.password or sftp.key_file is required")
	}
	for idx, file := range cfg.Files {
		if file.Local == "" {
			return fmt.Errorf("files[%d].local is required", idx)
		}
		if file.Remote == "" {
			return fmt.Errorf("files[%d].remote is required", idx)
		}
	}
	return nil
}

func loadConfig(path string) (*AppConfig, error) {
	var cfg AppConfig

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}

	err = cfg.validate()
	if err != nil {
		return nil, err
	}

	cfg.resolveLocalPaths(filepath.Dir(path))
	return &cfg, nil
}

// Check and convert relative paths to absolute paths
func (cfg *AppConfig) resolveLocalPaths(baseDir string) {
	for idx, file := range cfg.Files {
		if !filepath.IsAbs(file.Local) {
			cfg.Files[idx].Local = filepath.Join(baseDir, file.Local)
		}
	}
}

// Get the config file to load
func resolveConfigPath(cliPath string) (string, error) {
	var defaultConfigName = "default.yaml"
	var candidates []string

	// Specific config path given on the command line always wins
	if cliPath != "" {
		return filepath.Abs(cliPath)
	}

	// Search for default.yaml next to the executable
	if exePath, err := os.Executable(); err == nil {
		if exePath, err = filepath.EvalSymlinks(exePath); err == nil {
			candidates = append(candidates, filepath.Join(filepath.Dir(exePath), defaultConfigName))
		}
	}

	// Search for default.yaml in the current directory, useful when running from source.
	// With "go run" the executable is in a temp directory...
	if cwd, err := os.Getwd(); err == nil {
		cwdPath := filepath.Join(cwd, defaultConfigName)
		if len(candidates) == 0 || candidates[0] != cwdPath {
			candidates = append(candidates, cwdPath)
		}
	}

	// Find the first candidate that exists
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	// No valid config file found
	return "", fmt.Errorf("%s not found, searched:\n  - %s", defaultConfigName, strings.Join(candidates, "\n  - "))
}
