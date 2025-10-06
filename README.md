# Hellfire - Debian Router Configuration System

A UCI-like configuration management system for Debian routers, inspired by OpenWrt. Built in Go with event bus architecture, REST API, and modern React web UI.

## Features

- **UCI-style Configuration**: Human-readable config files similar to OpenWrt
- **Staging & Commit**: Test changes before applying them
- **Event Bus**: Pub/sub system for configuration changes
- **CLI Tool**: Command-line interface for managing configurations
- **REST API**: Web API with OpenAPI/Swagger documentation
- **Modern Web UI**: React 19 + TanStack Router + Type-safe API client
- **Handlers**: Automatic application of network and firewall configs

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│   CLI Tool  │────▶│Config Manager│────▶│  Event Bus  │
└─────────────┘     └──────────────┘     └─────────────┘
                           │                     │
                           ▼                     ▼
                    ┌─────────────┐     ┌──────────────┐
                    │ UCI Parser  │     │   Handlers   │
                    └─────────────┘     │ - Network    │
                           │            │ - Firewall   │
                           ▼            └──────────────┘
                    ┌─────────────┐            │
                    │/etc/config/ │            ▼
                    └─────────────┘     ┌──────────────┐
                                        │   System     │
┌─────────────┐                         │ - nftables   │
│  REST API   │─────────────────────────│ - ip/route   │
└─────────────┘                         └──────────────┘
```

## Installation

```bash
# Build the CLI tool
just build

# Or manually
go build -o bin/hf ./cmd/hf

# Install system-wide (optional)
just install
```

## Configuration Format

Configuration files are stored in `/etc/config/` using UCI format:

```
config interface 'wan'
    option proto 'static'
    option ipaddr '192.168.1.1'
    option netmask '255.255.255.0'
    list dns '8.8.8.8'
    list dns '1.1.1.1'

config rule
    option name 'ssh'
    option target 'ACCEPT'
    option proto 'tcp'
    option dest_port '22'
```

### Syntax

- `config <type> ['name']` - Defines a section
- `option 'key' 'value'` - Single-value option
- `list 'key' 'value'` - Multi-value list
- `# comment` - Comments

## CLI Usage

### View Configuration

```bash
# Show entire config file
hf show network
hf show firewall

# Get specific value
hf get network.wan.ipaddr
# Output: 192.168.1.1
```

### Modify Configuration

```bash
# Set a value (staged, not applied)
hf set network.wan.ipaddr 192.168.1.100
hf set firewall.ssh.enabled true

# View staged changes
hf changes

# Commit changes (apply to system)
hf commit

# Revert uncommitted changes
hf revert
```

### Export Configuration

```bash
# Export to file
hf export network > /backup/network.conf

# Import (manual copy)
cp /backup/network.conf /etc/config/network
hf commit
```

## Web UI

Hellfire includes a modern, type-safe web interface built with React 19, TanStack Router, and Tailwind CSS.

### Quick Start

```bash
# First time setup
just web-install          # Install dependencies
just web-generate-client  # Generate type-safe API client

# Development (runs backend + frontend together)
just dev

# Production build
just build-all-full       # Build both backend and web UI
./bin/hf serve --port 8080
# Visit http://localhost:8080
```

### Features

- **File-based Routing**: Add a route by creating a single file
- **Type-safe API Client**: Auto-generated from OpenAPI spec
- **Modern Stack**: React 19, TanStack Router, TanStack Query
- **shadcn/ui Components**: Copy/paste, fully customizable
- **Dark Mode Ready**: Tailwind CSS with theme support

See [web/README.md](web/README.md) for detailed documentation.

## REST API

### Start API Server

```bash
# Start on default port 8080
hf serve

# Start on custom port
hf serve --port 9000
```

### API Documentation

- **Swagger UI**: `http://localhost:8080/api/docs`
- **OpenAPI Spec**: `http://localhost:8080/api/openapi.json`

### API Endpoints

#### Get Configuration

```bash
# Get entire config
curl http://localhost:8080/api/config/network

# Get specific section
curl http://localhost:8080/api/config/network/wan

# Get specific option
curl http://localhost:8080/api/config/network/wan/ipaddr
```

#### Set Configuration

```bash
# Set an option (staged)
curl -X PUT http://localhost:8080/api/config/network/wan/ipaddr \
  -H "Content-Type: application/json" \
  -d '{"value": "192.168.1.100"}'
```

#### Commit/Revert Changes

```bash
# View staged changes
curl http://localhost:8080/api/changes

# Commit changes
curl -X POST http://localhost:8080/api/commit

# Revert changes
curl -X POST http://localhost:8080/api/revert
```

#### Health Check

```bash
curl http://localhost:8080/health
```

## Configuration Examples

See `examples/config/` for complete configuration examples:

- `network` - Network interfaces, routes, DNS
- `firewall` - Firewall rules, zones, forwarding
- `dhcp` - DHCP server and DNS (dnsmasq)
- `system` - System settings, hostname, timezone

### Network Configuration

```
config interface 'wan'
    option proto 'static'
    option ipaddr '192.168.1.1'
    option netmask '255.255.255.0'
    option gateway '192.168.1.254'
    list dns '8.8.8.8'
    list dns '1.1.1.1'

config interface 'lan'
    option proto 'static'
    option ipaddr '10.0.0.1'
    option netmask '255.255.255.0'
```

### Firewall Configuration

```
config defaults
    option input 'ACCEPT'
    option output 'ACCEPT'
    option forward 'DROP'

config zone
    option name 'wan'
    list network 'wan'
    option masq '1'

config rule
    option name 'Allow-SSH'
    option src 'wan'
    option proto 'tcp'
    option dest_port '22'
    option target 'ACCEPT'
```

### DHCP/DNS Configuration

```
config dnsmasq
    option domainneeded '1'
    option localise_queries '1'
    option local '/lan/'
    option domain 'lan'
    option expandhosts '1'
    option authoritative '1'

config dhcp 'lan'
    option interface 'lan'
    option start '100'
    option limit '150'
    option leasetime '12h'
    list dhcp_option '6,10.0.0.1'

config dhcp 'wan'
    option interface 'wan'
    option ignore '1'
```

## Event Bus

The event bus allows handlers to react to configuration changes:

```go
import "github.com/thesabbir/hellfire/pkg/bus"

// Subscribe to events
bus.Subscribe(bus.EventConfigCommitted, func(event bus.Event) {
    fmt.Println("Config committed:", event.ConfigName)
})

// Publish events
bus.Publish(bus.Event{
    Type: bus.EventConfigChanged,
    ConfigName: "network",
    Data: map[string]string{"key": "value"},
})
```

### Event Types

- `config.changed` - Configuration staged
- `config.committed` - Configuration committed
- `config.reverted` - Configuration reverted

## Handlers

Handlers automatically apply configuration changes to the system.

### Network Handler

Applies network interface configurations:

- Static IP addressing
- DHCP client
- Routes and gateways
- DNS servers

### Firewall Handler

Generates and applies nftables rules:

- Input/output/forward policies
- Firewall zones
- Port forwarding
- NAT/masquerading

### DHCP Handler

Manages DHCP server and DNS (dnsmasq):

- DHCP address pools
- DNS server configuration
- Domain configuration
- DHCP options

## Development

### Project Structure

```
hellfire/
├── cmd/
│   └── hf/               # CLI tool
│       ├── main.go       # CLI commands
│       └── api.go        # REST API server
├── pkg/
│   ├── uci/              # UCI parser
│   │   ├── types.go
│   │   ├── parser.go
│   │   └── parser_test.go
│   ├── config/           # Config manager
│   │   └── manager.go
│   ├── bus/              # Event bus
│   │   └── bus.go
│   └── handlers/         # Config handlers
│       ├── firewall.go
│       └── network.go
├── examples/
│   └── config/           # Example configs
└── go.mod
```

### Running Tests

```bash
# Run all tests
go test ./...

# Test UCI parser
go test ./pkg/uci -v
```

### Building

```bash
# Build for current platform
just build

# Build for all platforms (native, ARM64, AMD64)
just build-all

# Or manually
go build -o bin/hf ./cmd/hf
GOOS=linux GOARCH=arm64 go build -o bin/hf-linux-arm64 ./cmd/hf
GOOS=linux GOARCH=amd64 go build -o bin/hf-linux-amd64 ./cmd/hf
```

## Deployment

### On Debian/Ubuntu Router

1. Copy binary to router:

```bash
scp bin/hf-linux-arm64 user@router:/tmp/hf
```

2. Install and setup:

```bash
ssh user@router
sudo mv /tmp/hf /usr/local/bin/
sudo chmod +x /usr/local/bin/hf

# Create config directory
sudo mkdir -p /etc/config

# Copy example configs
sudo cp examples/config/* /etc/config/
```

3. Run as service (optional):

```bash
# Create systemd service
sudo tee /etc/systemd/system/hellfire-api.service <<EOF
[Unit]
Description=Hellfire Router Configuration API
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/hf serve --port 8080
Restart=always

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl enable hellfire-api
sudo systemctl start hellfire-api
```

## Use with Lima VM

The included `debian-router.yaml` is a Lima configuration for running a Debian router VM:

```bash
# Start the VM
limactl start debian-router.yaml

# Access the VM
limactl shell debian-router

# Inside VM, build and run
cd /Users/sabbir/Workspace/hellfire
go build -o hf ./cmd/hf
sudo ./hf serve
```

## Docker Deployment

Hellfire includes a Docker setup with systemd support for running as a containerized router.

### Quick Start

```bash
# Build and start the router
docker-compose up -d --build

# Check logs
docker-compose logs -f router

# Access the API
curl http://localhost:8888/health

# Enter the container
docker exec -it hellfire-router bash

# Stop the router
docker-compose down
```

### Features

- **Systemd Support**: Full systemd integration in container
- **Persistent Data**: Database and audit logs stored in Docker volume
- **Network Isolation**: Separate network for router and test clients
- **Privileged Mode**: Required for network management (nftables, routing)

### Architecture

The Docker setup includes:

- **Router Container**: Debian with systemd, runs Hellfire API and services
- **Client Container**: Test client with networking tools for testing
- **Persistent Volume**: `/var/lib/hellfire` for database and audit logs
- **Config Volume**: `/etc/config` mounted from `examples/config/`

### Configuration

Default configuration includes:

- **API Port**: 8080 (mapped to host 8888)
- **CORS**: Enabled for localhost:5173 and router.local
- **Password Policy**: 12 char minimum with complexity requirements
- **Session Timeout**: 24 hours (idle), 7 days (absolute)
- **Rate Limiting**: 100 req/min global, 5 req/min for auth
- **Audit Retention**: 90 days

Edit `examples/config/hellfire` to customize settings.

### Testing

```bash
# Enter the test client
docker exec -it hellfire-client bash

# Test connectivity
ping hellfire-router
curl http://hellfire-router:8080/health

# Test with networking tools
traceroute hellfire-router
nmap hellfire-router
```

### Accessing Services

From the router container:

```bash
# Check service status
systemctl status hellfire-api
systemctl status dnsmasq

# View logs
journalctl -u hellfire-api -f

# Manage services
systemctl restart hellfire-api
```

### Persistent Data

Database and audit logs are stored in the `hellfire-data` Docker volume:

```bash
# List volumes
docker volume ls

# Inspect volume
docker volume inspect hellfire_hellfire-data

# Backup volume
docker run --rm -v hellfire_hellfire-data:/data -v $(pwd):/backup \
  alpine tar czf /backup/hellfire-backup.tar.gz /data

# Restore volume
docker run --rm -v hellfire_hellfire-data:/data -v $(pwd):/backup \
  alpine tar xzf /backup/hellfire-backup.tar.gz -C /
```

## License

Copyright (C) 2025 Sabbir Ahmed

This project is licensed under the GNU General Public License v3.0 - see the [LICENSE](LICENSE) file for details.

## Contributing

Contributions welcome! Please open an issue or PR.
