# Exfil
Golang CLI that uploads local files to a remote SFTP server.

## Features
- Uploads files to a remote SFTP server
- SFTP connection with password or private key authentication
- Easy to configure with a simple YAML file
- Configurable error handling and overwrite options
- Dry-run mode to check connection and file selection without transferring files
- Cross-platform: Linux, Windows, macOS
- Single binary with no external dependencies

## Requirements
- Go 1.25 or later

## Build
The standard command for building with Go:
```bash
go build -o exfil .
```

Cross-compiling for another platform:
```bash
GOOS=windows GOARCH=amd64 go build -o exfil.exe .
GOOS=linux GOARCH=arm go build -o exfil .
```

## Usage
Command line parameters:
```bash
./exfil                         # reads config.yaml next to the executable then starts uploading files
./exfil --config /path/to/file  # alternative config file (also -c)
./exfil --dry-run               # check the SFTP connection and local files, doesn't transfer anything
```

## Configuration
The configuration file is in YAML format. See `config.example.yaml` for a sample configuration.
By default the script looks for `config.yaml` in the same directory as the executable.
A different file can be passed with `--config` or simply `-c`.

## Authentication
SFTP connection can be made with either a password or a private key file.
If both are set, `key_file` takes precedence over `password`.

## Error handling and overwrite
Use `exit_on_error` if the upload process should stop immediately after the first
failed transfer.  Otherwise, let it skip and continue with the remaining files.

Use `overwrite` if remote files should always be replaced. Otherwise any file already
present on the remote server is left untouched and skipped.

## Known hosts
The `only_known_hosts` option controls host key verification: `true` checks the server's
key against `~/.ssh/known_hosts` while `false` accepts any host key without verification.

Go's SSH library and OpenSSH negotiate host key algorithms in a different order of
preference (Go favors ECDSA/RSA, OpenSSH favors ed25519).  Because of this, a host key
saved by a manual ssh connection may only cover the algorithm that connection happened to
negotiate.  If connection fails with a key mismatch error, add all of the server's host
keys at once with:

```bash
ssh-keyscan -H [YOUR_SFTP_SERVER] >> ~/.ssh/known_hosts
```
