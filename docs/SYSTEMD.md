# Systemd Integration

This document explains how to run Hellfire with systemd for proper service management.

## Why Systemd?

Systemd provides:
- **Service management** - Start, stop, restart, reload services
- **Automatic restarts** - Services restart on failure
- **Dependency management** - Services start in correct order
- **Logging** - Centralized logging with journald
- **Future-proof** - Preparation for custom Debian OS build

## Quick Start

### Using Docker with Systemd

```bash
# Build and run with systemd
docker-compose -f docker-compose.systemd.yml up -d

# Check systemd status inside container
docker exec hellfire-router-systemd systemctl status

# View service status
docker exec hellfire-router-systemd systemctl status hellfire-api
docker exec hellfire-router-systemd systemctl status hellfire-firewall
docker exec hellfire-router-systemd systemctl status dnsmasq

# View logs
docker exec hellfire-router-systemd journalctl -u hellfire-api -f
docker exec hellfire-router-systemd journalctl -u hellfire-firewall -f

# Restart a service
docker exec hellfire-router-systemd systemctl restart dnsmasq
```

## Systemd Services

### hellfire-api.service
Main API server for managing router configuration.

**Commands:**
```bash
systemctl start hellfire-api
systemctl stop hellfire-api
systemctl restart hellfire-api
systemctl status hellfire-api
```

### hellfire-network.service
Network configuration service (oneshot, runs at boot).

**Commands:**
```bash
systemctl start hellfire-network  # Apply network config
systemctl stop hellfire-network   # Take down network
systemctl status hellfire-network
```

### hellfire-firewall.service
Firewall management service (oneshot, runs at boot).

**Commands:**
```bash
systemctl start hellfire-firewall   # Apply firewall rules
systemctl reload hellfire-firewall  # Reload rules without stopping
systemctl stop hellfire-firewall    # Flush all rules
systemctl status hellfire-firewall
```

### dnsmasq.service
Built-in Debian service for DHCP/DNS.

**Commands:**
```bash
systemctl restart dnsmasq
systemctl status dnsmasq
```

## Service Dependencies

```
hellfire-network.service
    ↓
hellfire-firewall.service (requires network)
    ↓
hellfire-api.service
dnsmasq.service
```

## Logs and Debugging

### View all logs
```bash
journalctl -xe
```

### Follow specific service
```bash
journalctl -u hellfire-api -f
```

### View boot logs
```bash
journalctl -b
```

### Filter by time
```bash
journalctl --since "10 minutes ago"
journalctl --since "2024-01-01" --until "2024-01-02"
```

## Docker Compose Configuration

The systemd setup requires special Docker configuration:

- **privileged: true** - Required for systemd
- **SYS_ADMIN capability** - For systemd operations
- **cgroup mount** - Systemd needs access to cgroups
- **seccomp=unconfined** - Remove security restrictions for systemd
- **SIGRTMIN+3** - Proper shutdown signal for systemd

## Comparison: Standard vs Systemd

| Feature | Standard Dockerfile | Systemd Dockerfile |
|---------|-------------------|-------------------|
| Init system | None (direct exec) | systemd |
| Service management | Manual | systemctl |
| Auto-restart | Docker restart policy | systemd |
| Logging | Docker logs | journald |
| Multiple services | Not managed | Fully managed |
| Production-ready | Development | Production-like |
| Image size | ~200MB | ~300MB |
| Startup time | ~1s | ~3-5s |

## Using on Real Hardware / Custom OS

When building a custom Debian-based router OS:

1. **Package your binary:**
   ```bash
   # Create .deb package
   fpm -s dir -t deb -n hellfire -v 1.0.0 \
     --prefix /usr/local/bin hf
   ```

2. **Install services:**
   ```bash
   cp systemd/*.service /etc/systemd/system/
   systemctl daemon-reload
   systemctl enable hellfire-api
   systemctl enable hellfire-network
   systemctl enable hellfire-firewall
   ```

3. **Start services:**
   ```bash
   systemctl start hellfire-api
   ```

## Troubleshooting

### Container won't start
```bash
# Check if cgroups are available
docker run --rm --privileged debian:trixie-slim ls /sys/fs/cgroup

# Try with more verbose logging
docker-compose -f docker-compose.systemd.yml logs -f
```

### Services failing
```bash
# Check service status
docker exec hellfire-router-systemd systemctl --failed

# View detailed logs
docker exec hellfire-router-systemd journalctl -xeu hellfire-api
```

### Permission issues
Ensure the container has:
- `privileged: true`
- `SYS_ADMIN` capability
- Proper cgroup mounts

## Next Steps

1. **Implement subcommands** - Add `hf network apply`, `hf firewall apply`, etc.
2. **Create .deb packages** - Package for Debian installation
3. **Build custom ISO** - Use `live-build` for custom Debian image
4. **Hardware deployment** - Flash to router hardware

## References

- [systemd documentation](https://systemd.io/)
- [Running systemd in Docker](https://developers.redhat.com/blog/2019/04/24/how-to-run-systemd-in-a-container)
- [Debian live-build](https://wiki.debian.org/DebianLive)
