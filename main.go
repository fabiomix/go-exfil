package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	var configPath string
	var isDryRun bool

	// Define command-line flags
	flag.StringVar(&configPath, "config", "", "Path to the configuration file")
	flag.StringVar(&configPath, "c", "", "Path to the configuration file (alias for --config)")
	flag.BoolVar(&isDryRun, "dry-run", false, "Perform a dry run without transferring files")
	flag.Parse()

	// Configuration file path is optional
	configPath, err := resolveConfigPath(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
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
	okCount, skippedCount, failedCount := 0, 0, 0
	for _, res := range results {
		label := fmt.Sprintf("%-6s", "["+res.Status+"]")
		if res.Message == "" {
			fmt.Printf(" %s %s  ->  %s\n", label, res.FileEntry.Local, res.FileEntry.Remote)
		} else {
			fmt.Printf(" %s %s  ->  %s: %s\n", label, res.FileEntry.Local, res.FileEntry.Remote, res.Message)
		}

		switch res.Status {
		case StatusOK:
			okCount++
		case StatusSkipped:
			skippedCount++
		case StatusFailed:
			failedCount++
		default:
			failedCount++
		}
	}

	// Files never attempted (in case of exit_on_error)
	notAttempted := len(cfg.Files) - len(results)
	skippedCount += notAttempted

	summary := fmt.Sprintf("Results: total=%d, ok=%d, skipped=%d, failed=%d", len(cfg.Files), okCount, skippedCount, failedCount)
	if isDryRun {
		summary += " (dry-run mode, no files were transferred)"
	}
	fmt.Printf("\n%s\n", summary)

	// Exit code based on the number of failed transfers
	if failedCount > 0 {
		os.Exit(3)
	} else {
		os.Exit(0)
	}
}
