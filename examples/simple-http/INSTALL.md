# Installation Guide

## Required Dependencies

The test script requires the following tools:

| Tool     | Purpose             | Installation                                     |
| -------- | ------------------- | ------------------------------------------------ |
| **go**   | Compile Go programs | https://go.dev/doc/install                       |
| **k6**   | Load testing        | https://k6.io/docs/getting-started/installation/ |
| **jq**   | JSON parsing        | `brew install jq` or `apt install jq`            |
| **bc**   | Math calculations   | `brew install bc` or `apt install bc`            |
| **curl** | HTTP requests       | Usually pre-installed                            |

## Quick Install

### macOS (Homebrew)

```bash
brew install go k6 jq bc
```

### Linux (Debian/Ubuntu)

```bash
# Go
wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# k6
sudo gpg -k
sudo gpg --no-default-keyring --keyring /usr/share/keyrings/k6-archive-keyring.gpg --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D69
echo "deb [signed-by=/usr/share/keyrings/k6-archive-keyring.gpg] https://dl.k6.io/deb stable main" | sudo tee /etc/apt/sources.list.d/k6.list
sudo apt-get update
sudo apt-get install k6

# Other tools
sudo apt install jq bc curl
```

### Linux (RHEL/Fedora/CentOS)

```bash
# Go
wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# k6
sudo dnf install https://dl.k6.io/rpm/repo.rpm
sudo dnf install k6

# Other tools
sudo dnf install jq bc curl
```

### Windows (WSL2 recommended)

Use WSL2 and follow the Linux instructions above, or:

```powershell
# Using Chocolatey
choco install golang k6 jq

# bc alternative: PowerShell has built-in math
# curl alternative: PowerShell Invoke-WebRequest
```

## Verification

Run the test script with dependency check:

```bash
cd examples/simple-http
bash test.sh
```

If dependencies are missing, you'll see:

```
❌ Missing required dependencies: k6 jq bc
```

With helpful installation instructions for each.

## Troubleshooting

### Go not in PATH

After installing Go, add to your shell profile:

```bash
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

### k6 permission denied

```bash
sudo chmod +x /usr/local/bin/k6
```

### jq/bc not found

Make sure your package manager is up to date:

```bash
# macOS
brew update

# Linux
sudo apt update
# or
sudo dnf update
```

## Minimal System Requirements

- **CPU**: 2+ cores (4+ recommended for load testing)
- **RAM**: 4GB minimum (8GB+ recommended)
- **Disk**: 1GB free space
- **Network**: Localhost testing (no internet required for basic tests)

## Next Steps

Once dependencies are installed:

1. Run the comparison test: `bash test.sh`
2. Watch the forensic output
3. See the 10x latency improvement proof
4. Integrate lawbench into your service

## Getting Help

If you encounter issues:

1. Check dependency versions: `go version`, `k6 version`, `jq --version`
2. Verify script syntax: `bash -n test.sh`
3. Run with debug: `bash -x test.sh` (shows each command)
4. Open an issue: https://github.com/alexshd/trdynamics/issues
