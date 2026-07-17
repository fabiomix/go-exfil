package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"gopkg.in/yaml.v3"
)

// === Config ===

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

type UploadResult struct {
	FileEntry FileEntry // or local+remote as strings
	Ok        bool
	Message   string
}

// === Class Methods ===

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

// === Helpers ===

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
	// TODO: get the config.yaml path in the same path as the executable, not the current working directory
	return filepath.Join(filepath.Dir(os.Args[0]), "config.yaml")
}

// === Modes ===

func dryRunMode(cfg *AppConfig) ([]UploadResult, error) {
	results := make([]UploadResult, 0, len(cfg.Files))

	sshClient, err := cfg.SFTP.getSSHClient()
	if err != nil {
		return nil, err
	}
	defer sshClient.Close()

	for idx, file := range cfg.Files {
		res := UploadResult{FileEntry: file}
		fp, err := os.Stat(file.Local)
		if err != nil {
			res.Ok = false
			res.Message = fmt.Sprintf("files[%d].local: %v", idx, err)
		} else if fp.IsDir() {
			res.Ok = false
			res.Message = fmt.Sprintf("files[%d].local: is a directory", idx)
		} else {
			res.Ok = true
			res.Message = "OK"
		}
		results = append(results, res)
	}

	return results, nil
}

func transferMode(cfg *AppConfig) ([]UploadResult, error) {
	results := make([]UploadResult, 0, len(cfg.Files))

	sshClient, err := cfg.SFTP.getSSHClient()
	if err != nil {
		return nil, err
	}
	defer sshClient.Close()

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return nil, err
	}
	defer sftpClient.Close()

	for _, file := range cfg.Files {
		res := sendFile(sftpClient, file, cfg.Overwrite)
		results = append(results, res)
		if !res.Ok && cfg.ExitOnError {
			break
		}
	}

	return results, nil
}

func sendFile(client *sftp.Client, file FileEntry, overwrite bool) UploadResult {
	res := UploadResult{FileEntry: file}

	// When overwrite is false, check if the remote file exists
	if !overwrite {
		remoteStat, err := client.Stat(file.Remote)
		if err == nil && remoteStat != nil {
			// TODO: when file already exists and overwrite is false, res.Ok should be true or false?
			// It also depends on cfg.ExitOnError, because skipped files are not errors when overwrite is false
			res.Ok = false
			res.Message = "skipped (remote file exists)"
			return res
		}
	}

	// Read the local file
	sourceFile, err := os.Open(file.Local)
	if err != nil {
		res.Ok = false
		res.Message = fmt.Sprintf("opening local file: %v", err)
		return res
	}
	defer sourceFile.Close()

	// Create the remote directory if it doesn't exist
	err = client.MkdirAll(filepath.Dir(file.Remote))
	if err != nil {
		res.Ok = false
		res.Message = fmt.Sprintf("creating remote directory: %v", err)
		return res
	}

	// Touch the remote file
	// sftp.Create always truncates an existing file!
	destFile, err := client.Create(file.Remote)
	if err != nil {
		res.Ok = false
		res.Message = fmt.Sprintf("creating remote file: %v", err)
		return res
	}
	defer destFile.Close()

	// Copy the contents of the local file to the remote file
	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		res.Ok = false
		res.Message = fmt.Sprintf("copying file: %v", err)
		return res
	}

	// Everything went well
	res.Ok = true
	res.Message = ""
	return res
}

// === Main ===

func main() {
	var configPath string
	var isDryRun bool

	// Define command-line flags
	flag.StringVar(&configPath, "config", "", "Path to the configuration file")
	flag.StringVar(&configPath, "c", "", "Path to the configuration file (alias for --config)")
	flag.BoolVar(&isDryRun, "dry-run", false, "Perform a dry run without transferring files")
	flag.Parse()

	// Configuration file path is optional
	if configPath == "" {
		configPath = defaultConfigPath()
	}

	// Load the configuration
	cfg, err := loadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Print the run header before attempting the connection
	fmt.Printf("go-exfil // upload local files to a remote SFTP server\n")
	fmt.Printf("Config: %s\n", configPath)
	fmt.Printf("Params: overwrite=%t, exit_on_error=%t, dry_run=%t\n", cfg.Overwrite, cfg.ExitOnError, isDryRun)

	// Run the appropriate mode based on the dry-run flag
	var results []UploadResult
	if isDryRun {
		results, err = dryRunMode(cfg)
	} else {
		results, err = transferMode(cfg)
	}

	// Handle errors from the mode execution (connection failures land here)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(2)
	} else {
		fmt.Printf("Successfully connected to %s:%d (user: %s)\n\n", cfg.SFTP.Host, cfg.SFTP.Port, cfg.SFTP.User)
	}

	// Print the results of the file transfers
	// TODO: print results after each file transfer, not at the end, to provide real-time feedback
	failed := 0
	for _, res := range results {
		if res.Ok {
			fmt.Printf(" [OK]  %s  ->  %s\n", res.FileEntry.Local, res.FileEntry.Remote)
		} else {
			fmt.Printf(" [ERR] %s  ->  %s: %s\n", res.FileEntry.Local, res.FileEntry.Remote, res.Message)
			failed++
		}
	}

	summary := fmt.Sprintf("Results: ok=%d, failed=%d, total=%d", len(results)-failed, failed, len(cfg.Files))
	if isDryRun {
		summary += " (dry-run mode, no files were transferred)"
	}
	fmt.Printf("\n%s\n", summary)

	// Exit code based on the number of failed transfers
	if failed > 0 {
		os.Exit(3)
	} else {
		os.Exit(0)
	}
}
