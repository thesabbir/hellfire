# Quick Start Guide

## Quick Test with Docker

```bash
# Build and run router + client containers
just docker

# Test the API
just docker-test

# Shell into client to test routing
just docker-shell-client

# Shell into router to configure
just docker-shell-router

# Stop containers
just docker-down
```

This creates a router and client container on a shared network for testing. Access the API on http://localhost:8888

### Testing from Client

```bash
# Shell into client
just docker-shell-client

# Test connectivity
ping -c 3 hellfire-router

# Test DNS resolution
dig @hellfire-router example.com

# Test API from inside network
curl http://hellfire-router:8080/health

# Network scanning
nmap hellfire-router

# Check routing
traceroute hellfire-router
ip route show
```

### Testing Tools Available

**Client container includes:**
- `curl`, `wget` - HTTP clients
- `ping`, `traceroute`, `mtr` - Network diagnostics
- `dig`, `nslookup` - DNS tools
- `nmap` - Network scanner
- `tcpdump` - Packet capture
- `iperf3` - Bandwidth testing
- `netcat`, `socat` - TCP/UDP testing
- `nftables`, `iptables` - Firewall inspection

## Fix asdf Go version (if needed)

If you see version mismatch errors, your asdf shim is using wrong GOROOT:

```bash
# Option 1: Use specific Go version directly
export GOROOT=$HOME/.asdf/installs/golang/1.25.1/go
export PATH="$GOROOT/bin:$PATH"

# Option 2: Reinstall current Go version with asdf
asdf uninstall golang 1.25.1
asdf install golang 1.25.1
asdf reshim golang
```

## Build and Test

```bash
# Run tests
just test

# Build binary
just build

# Build for all platforms
just build-all
```

## Try It Out

```bash
# Use example configs (requires sudo for /etc/config)
sudo mkdir -p /etc/config
sudo cp examples/config/* /etc/config/

# View config
./bin/hf show network

# Get a value
./bin/hf get network.wan.ipaddr

# Set a value (staged)
./bin/hf set network.wan.ipaddr 192.168.1.100

# View changes
./bin/hf changes

# Commit changes
./bin/hf commit

# Revert changes
./bin/hf revert
```

## Start API Server

```bash
# Start on port 8888
just serve

# Or manually
./bin/hf serve --port 8888

# Test API
curl http://localhost:8888/health
curl http://localhost:8888/api/config/network
```

## Test Without Root

```bash
# Use custom config directory (no sudo needed)
./bin/hf --config-dir ./examples/config show network
./bin/hf --config-dir ./examples/config get network.wan.ipaddr
```

## Project Structure

```
hellfire/
├── cmd/hf/           # CLI tool
│   ├── main.go       # Commands
│   └── api.go        # REST API
├── pkg/
│   ├── uci/          # UCI parser
│   ├── config/       # Config manager
│   ├── bus/          # Event bus
│   └── handlers/     # System handlers
├── examples/config/  # Sample configs
├── justfile         # Build commands
└── README.md        # Full documentation
```

## Common Commands

```bash
just build              # Build binary
just test               # Run tests
just serve              # Start API server
just examples           # Show usage examples
just clean              # Remove binaries
```
