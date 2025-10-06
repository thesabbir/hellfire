# Hellfire Architecture

## Overview

Hellfire is a transaction-based router configuration management system inspired by OpenWrt's UCI, VyOS, and NixOS. It provides a simple, coherent, and powerful way to manage Debian-based routers.

## Design Philosophy

### Simple
- File-based snapshots (no git complexity)
- Clear command structure (`hf set`, `hf commit`, `hf rollback`)
- UCI-like configuration format (familiar to router admins)
- One binary, minimal dependencies

### Coherent
- Everything follows the transaction model
- All subsystems use the same applier pattern
- Event bus ties components together
- UCI configs are the single source of truth

### Powerful
- Atomic operations (all-or-nothing)
- Auto-rollback on failure
- Confirm-or-revert prevents lockout
- Point-in-time recovery
- Production-ready for custom OS deployment

## Core Components

### 1. UCI Parser (`pkg/uci/`)
Parses OpenWrt-style UCI configuration files.

```
config interface 'wan'
    option proto 'static'
    option ipaddr '192.168.1.1'
    option netmask '255.255.255.0'
```

### 2. Config Manager (`pkg/config/`)
Manages configuration files with staging support:
- Load/Save UCI files
- Staging area for uncommitted changes
- Dot-notation access (`network.wan.ipaddr`)

### 3. Snapshot Manager (`pkg/snapshot/`)
File-based snapshot system:
- Creates timestamped copies of configs
- Metadata tracking (timestamp, message, changed configs)
- List, restore, and prune operations
- Stored in `/var/lib/hellfire/snapshots/`

### 4. Appliers (`pkg/appliers/`)
Components that apply configurations to the system:

**Applier Interface:**
```go
type Applier interface {
    Name() string
    Apply(config *uci.Config) error
    Validate() error
    Rollback() error
}
```

**Implementations:**
- `NetworkApplier`: Configures interfaces using `ip` commands
- `FirewallApplier`: Generates and applies nftables rules
- `DHCPApplier`: Generates dnsmasq configs and restarts service

### 5. Transaction Manager (`pkg/transaction/`)
Orchestrates atomic configuration changes:

**Transaction Flow:**
1. Create snapshot of current state
2. Commit staged changes to disk
3. Apply configurations in order (network → firewall → dhcp)
4. Validate each step
5. If confirm-timeout set, wait for confirmation
6. On error: automatic rollback

**States:**
- `idle`: No transaction in progress
- `in_progress`: Applying changes
- `pending`: Waiting for confirmation
- `completed`: Success
- `failed`: Rolled back

### 6. Event Bus (`pkg/bus/`)
Pub/sub system for component communication:

**Events:**
- `config.changed`: Configuration staged
- `config.committed`: Configuration written
- `snapshot.created`: Snapshot created
- `transaction.started`: Transaction begins
- `transaction.completed`: Transaction succeeds
- `transaction.failed`: Transaction fails
- `rollback.started`: Rollback initiated

### 7. CLI (`cmd/hf/`)
Command-line interface built with Cobra:

**Config Management:**
- `hf show <config>`: Display configuration
- `hf get <path>`: Get specific value
- `hf set <path> <value>`: Stage change
- `hf changes`: Show staged changes

**Transactions:**
- `hf commit [-m msg] [-t timeout]`: Commit with optional confirm-timeout
- `hf confirm`: Confirm pending changes
- `hf rollback`: Rollback to previous snapshot

**Snapshots:**
- `hf snapshot list`: List all snapshots
- `hf snapshot restore <id>`: Restore snapshot
- `hf snapshot prune [--keep=N]`: Clean old snapshots

**Apply (for systemd):**
- `hf network apply`: Apply network config
- `hf firewall apply`: Apply firewall rules
- `hf dhcp apply`: Apply DHCP/DNS config

### 8. API Server (`cmd/hf/api.go`)
REST API built with Gin:
- Config CRUD operations
- Commit/revert endpoints
- Swagger documentation
- Web UI integration

## Data Flow

### Configuration Change Flow

```
User → CLI → Config Manager → Transaction Manager
                                      ↓
                                Snapshot Manager (create backup)
                                      ↓
                                Config Manager (commit to disk)
                                      ↓
                        Applier Registry (network, firewall, dhcp)
                                      ↓
                            System (kernel, services)
                                      ↓
                              Validation (check)
                                      ↓
                         Event Bus (notify subscribers)
```

### Rollback Flow

```
User → CLI → Transaction Manager
                    ↓
           Snapshot Manager (restore files)
                    ↓
           Applier Registry (re-apply configs)
                    ↓
           System (kernel, services)
```

## Directory Structure

```
hellfire/
├── cmd/
│   └── hf/
│       ├── main.go          # CLI entry point
│       └── api.go           # API server
│
├── pkg/
│   ├── uci/                 # UCI parser
│   │   ├── parser.go
│   │   └── types.go
│   │
│   ├── config/              # Config manager
│   │   └── manager.go
│   │
│   ├── snapshot/            # Snapshot manager
│   │   └── manager.go
│   │
│   ├── appliers/            # Applier implementations
│   │   ├── applier.go       # Interface + registry
│   │   ├── network.go       # Network applier
│   │   ├── firewall.go      # Firewall applier
│   │   └── dhcp.go          # DHCP applier
│   │
│   ├── transaction/         # Transaction manager
│   │   └── manager.go
│   │
│   ├── bus/                 # Event bus
│   │   └── bus.go
│   │
│   └── handlers/            # API handlers (legacy)
│       ├── network.go
│       ├── firewall.go
│       └── dhcp.go
│
├── systemd/                 # Systemd service files
│   ├── hellfire-api.service
│   ├── hellfire-network.service
│   ├── hellfire-firewall.service
│   └── hellfire-dhcp.service
│
├── examples/config/         # Example configurations
│   ├── network
│   ├── firewall
│   ├── dhcp
│   └── system
│
├── web/                     # Web UI (React)
│
└── docs/                    # Documentation
    ├── ARCHITECTURE.md
    ├── TRANSACTIONS.md
    └── SYSTEMD.md
```

## Runtime Directories

```
/etc/config/                          # Active configurations
├── network
├── firewall
├── dhcp
└── system

/tmp/uci-staging/                     # Staging area
├── network
├── firewall
└── dhcp

/var/lib/hellfire/snapshots/          # Snapshots
├── 20241006-153000/
│   ├── metadata.json
│   ├── network
│   └── firewall
└── 20241006-144500/
    ├── metadata.json
    └── dhcp
```

## Systemd Integration

### Service Dependency Order

```
hellfire-network.service (network config)
        ↓
hellfire-firewall.service (depends on network)
        ↓
hellfire-dhcp.service (depends on network)
        ↓
dnsmasq.service (depends on dhcp config)
        ↓
hellfire-api.service (API server)
```

### Service Types

**oneshot services** (network, firewall, dhcp):
- Run once at boot
- Apply configuration and exit
- `RemainAfterExit=yes` keeps them "active"

**long-running service** (api):
- Runs continuously
- Provides REST API
- Web UI access

## Comparison with Similar Systems

### vs. OpenWrt UCI

**Similarities:**
- UCI config format
- Staging/commit workflow
- Modular design

**Differences:**
- Hellfire adds transactions
- Automatic snapshots
- Confirm-or-revert safety
- Better rollback support

### vs. VyOS

**Similarities:**
- Commit/rollback/confirm workflow
- Configuration versioning
- Network device focus

**Differences:**
- Hellfire uses UCI format (simpler)
- File-based snapshots (vs boot images)
- Smaller footprint

### vs. NixOS

**Similarities:**
- Atomic operations
- Rollback support
- Declarative configuration

**Differences:**
- Hellfire is router-focused
- Simpler implementation
- UCI instead of Nix language

## Security Considerations

### File Permissions
- Configs: `/etc/config/` - root:root 0644
- Snapshots: `/var/lib/hellfire/` - root:root 0755
- Binary: `/usr/local/bin/hf` - root:root 0755

### API Security
- No authentication in current version (TODO)
- Should run behind reverse proxy
- Or use firewall rules to restrict access

### Systemd Security
- Services run as root (required for network config)
- Could use capabilities instead (future improvement)

## Performance

### Snapshot Creation
- Simple file copy: ~10ms for typical config
- Scales linearly with config size
- Prune old snapshots to manage disk space

### Transaction Apply
- Network: ~500ms (interface configuration)
- Firewall: ~100ms (nftables reload)
- DHCP: ~200ms (dnsmasq restart)
- Total: ~1 second for full apply

### Rollback
- Restore files: ~10ms
- Re-apply: ~1 second
- Total: ~1 second to rollback

## Future Enhancements

### Short-term
- [ ] Snapshot diffs
- [ ] Transaction status command
- [ ] Dry-run mode
- [ ] Better validation

### Medium-term
- [ ] API authentication
- [ ] Remote snapshots
- [ ] Scheduled backups
- [ ] Transaction history

### Long-term
- [ ] Custom Debian ISO builder
- [ ] Hardware support (ARM, x86)
- [ ] Cluster support
- [ ] WebUI improvements

## Testing Strategy

### Unit Tests
- UCI parser
- Config manager
- Snapshot manager
- Individual appliers

### Integration Tests
- Full transaction flow
- Rollback scenarios
- Confirm-timeout behavior

### System Tests
- Docker containers
- Real hardware (future)
- Custom OS builds (future)

## Deployment Options

### 1. Docker (Development)
```bash
docker-compose up
```

### 2. Docker with Systemd (Testing)
```bash
docker-compose -f docker-compose.systemd.yml up
```

### 3. Debian Package (Production)
```bash
dpkg -i hellfire_1.0.0_amd64.deb
systemctl enable hellfire-network
systemctl start hellfire-network
```

### 4. Custom OS Image (Future)
```bash
# Build custom Debian ISO with Hellfire
./build-iso.sh
# Flash to router hardware
```

## Contributing

See the main README for contribution guidelines.

## License

GPL v3 - See LICENSE file.
