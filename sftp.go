package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// TODO: not a class method, but a standalone function?
func (cfg *SFTPConfig) getSSHClientConfig() (*ssh.ClientConfig, error) {
	var authMethods []ssh.AuthMethod
	var hostKeyCallback ssh.HostKeyCallback

	if cfg.KeyFile != "" {
		key, err := os.ReadFile(cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("reading key file: %w", err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("parsing private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	} else if cfg.Password != "" {
		authMethods = append(authMethods, ssh.Password(cfg.Password))
	} else {
		return nil, fmt.Errorf("no authentication method provided")
	}

	if cfg.OnlyKnownHosts {
		homeDir, err := os.UserHomeDir() // TODO: shall we check for errors here?
		knownHostsFile := filepath.Join(homeDir, ".ssh", "known_hosts")
		hostKeyCallback, err = knownhosts.New(knownHostsFile)
		if err != nil {
			return nil, fmt.Errorf("loading known_hosts: %w", err)
		}
	} else {
		// accept any host key
		hostKeyCallback = ssh.InsecureIgnoreHostKey()
	}

	// algorithms := ssh.SupportedAlgorithms()
	return &ssh.ClientConfig{
		// Config: ssh.Config{
		// 	KeyExchanges: algorithms.KeyExchanges,
		// 	Ciphers:      algorithms.Ciphers,
		// 	MACs:         algorithms.MACs,
		// },
		User:            cfg.User,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         30 * time.Second,
	}, nil
}

// TODO: not a class method, but a standalone function?
func (cfg *SFTPConfig) getSSHClient() (*ssh.Client, error) {
	sshConfig, err := cfg.getSSHClientConfig()
	if err != nil {
		return nil, err
	}
	addr := net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port))
	return ssh.Dial("tcp", addr, sshConfig)
}
