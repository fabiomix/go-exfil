package main

import (
	"fmt"
	"os"
	"path/filepath"

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

	return &cfg, nil
}

func defaultConfigPath() string {
	// TODO: get the default.yaml path in the same path as the executable, not the current working directory
	return filepath.Join(filepath.Dir(os.Args[0]), "default.yaml")
}
