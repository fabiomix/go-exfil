package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/sftp"
)

// Status code and labels for upload results
const StatusOK string = "OK"
const StatusSkipped string = "SKIP"
const StatusFailed string = "ERR"

type UploadResult struct {
	FileEntry FileEntry // or local+remote as strings
	Status    string
	Message   string
}

// Print the result of the file transfer
func printResult(res UploadResult) {
	label := fmt.Sprintf("%-6s", "["+res.Status+"]")
	if res.Message == "" {
		fmt.Printf(" %s %s  ->  %s\n", label, res.FileEntry.Local, res.FileEntry.Remote)
	} else {
		fmt.Printf(" %s %s  ->  %s: %s\n", label, res.FileEntry.Local, res.FileEntry.Remote, res.Message)
	}
}

func dryRunMode(cfg *AppConfig) ([]UploadResult, error) {
	results := make([]UploadResult, 0, len(cfg.Files))

	sshClient, err := cfg.SFTP.getSSHClient()
	if err != nil {
		return nil, err
	}
	defer sshClient.Close()
	fmt.Printf("Successfully connected to %s:%d (user: %s)\n\n", cfg.SFTP.Host, cfg.SFTP.Port, cfg.SFTP.User)

	for idx, file := range cfg.Files {
		res := UploadResult{FileEntry: file}
		fp, err := os.Stat(file.Local)
		if err != nil {
			res.Status = StatusFailed
			res.Message = fmt.Sprintf("files[%d].local: %v", idx, err)
		} else if fp.IsDir() {
			res.Status = StatusFailed
			res.Message = fmt.Sprintf("files[%d].local: is a directory", idx)
		} else {
			res.Status = StatusOK
			res.Message = ""
		}
		printResult(res)
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
	fmt.Printf("Successfully connected to %s:%d (user: %s)\n\n", cfg.SFTP.Host, cfg.SFTP.Port, cfg.SFTP.User)

	for _, file := range cfg.Files {
		res := sendFile(sftpClient, file, cfg.Overwrite)
		printResult(res)
		results = append(results, res)
		if res.Status == StatusFailed && cfg.ExitOnError {
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
			res.Status = StatusSkipped
			res.Message = "remote file exists"
			return res
		}
	}

	// Read the local file
	sourceFile, err := os.Open(file.Local)
	if err != nil {
		res.Status = StatusFailed
		res.Message = fmt.Sprintf("opening local file: %v", err)
		return res
	}
	defer sourceFile.Close()

	// Create the remote directory if it doesn't exist
	err = client.MkdirAll(filepath.Dir(file.Remote))
	if err != nil {
		res.Status = StatusFailed
		res.Message = fmt.Sprintf("creating remote directory: %v", err)
		return res
	}

	// Touch the remote file
	// sftp.Create always truncates an existing file!
	destFile, err := client.Create(file.Remote)
	if err != nil {
		res.Status = StatusFailed
		res.Message = fmt.Sprintf("creating remote file: %v", err)
		return res
	}
	defer destFile.Close()

	// Copy the contents of the local file to the remote file
	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		res.Status = StatusFailed
		res.Message = fmt.Sprintf("copying file: %v", err)
		return res
	}

	// Everything went well
	res.Status = StatusOK
	res.Message = ""
	return res
}
