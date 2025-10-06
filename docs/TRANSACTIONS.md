# Transaction System

Hellfire implements a database-like transaction system for managing router configurations safely and reliably.

## Overview

The transaction system provides:
- **Atomic operations**: All changes succeed or all rollback
- **Automatic snapshots**: Every commit creates a checkpoint
- **Confirm-or-revert**: Prevents network lockout
- **Point-in-time recovery**: Restore any previous state

## Architecture

```
┌─────────────────────────────────────────────────┐
│                 UCI Configs                      │
│          (Desired State - Source of Truth)       │
└──────────────────┬──────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────┐
│            Transaction Manager                   │
│  • Auto-snapshot before changes                  │
│  • Apply all configs atomically                  │
│  • Rollback on failure                           │
│  • Confirm-or-revert (network safety)            │
└──────────────────┬──────────────────────────────┘
                   │
      ┌────────────┼────────────┐
      ▼            ▼            ▼
┌──────────┐ ┌──────────┐ ┌──────────┐
│ Network  │ │ Firewall │ │   DHCP   │
│ Applier  │ │ Applier  │ │ Applier  │
└──────────┘ └──────────┘ └──────────┘
      │            │            │
      ▼            ▼            ▼
┌─────────────────────────────────────────────────┐
│         System State (Kernel/Services)           │
│      (interfaces, nftables, dnsmasq)            │
└─────────────────────────────────────────────────┘
```

## Basic Workflow

### 1. Stage Changes

```bash
# Modify configuration
hf set network.wan.ipaddr 192.168.1.100
hf set network.wan.gateway 192.168.1.1

# View staged changes
hf changes
```

### 2. Commit Changes

```bash
# Simple commit (applies immediately)
hf commit -m "Update WAN IP address"

# Commit with confirmation timeout (safer for network changes)
hf commit -m "Update WAN IP address" --confirm-timeout=60
```

### 3. Confirm or Rollback

If you used `--confirm-timeout`, you have that many seconds to confirm:

```bash
# If changes work, confirm them
hf confirm

# If something went wrong, rollback immediately
hf rollback
```

If you don't confirm within the timeout, changes automatically rollback.

## Snapshots

Every commit automatically creates a snapshot. You can also manage snapshots manually.

### List Snapshots

```bash
hf snapshot list
```

Output:
```
Available snapshots:
1. 20241006-153000 - Update WAN IP address
   Time: 2024-10-06 15:30:00
   Configs: [network]

2. 20241006-144500 - Add firewall rules
   Time: 2024-10-06 14:45:00
   Configs: [firewall]

3. 20241006-120000 - Initial configuration
   Time: 2024-10-06 12:00:00
   Configs: [network firewall dhcp]
```

### Restore Snapshot

```bash
# Restore by snapshot ID
hf snapshot restore 20241006-144500

# This restores the files, then you need to commit to apply
hf commit -m "Restored to previous state"
```

### Prune Old Snapshots

```bash
# Keep only last 30 snapshots (default)
hf snapshot prune

# Keep only last 10 snapshots
hf snapshot prune --keep=10
```

## Advanced Usage

### Network Change with Safety Timer

This is the safest way to apply network changes that might lock you out:

```bash
# Make network changes
hf set network.wan.ipaddr 10.0.0.1
hf set network.wan.gateway 10.0.0.254

# Commit with 60 second confirmation window
hf commit -m "Change WAN to new subnet" --confirm-timeout=60

# Test your connection...
# If it works, confirm:
hf confirm

# If you get locked out, it will auto-rollback after 60 seconds
```

### Manual Rollback

```bash
# Rollback to previous snapshot immediately
hf rollback
```

This is useful if:
- You noticed a problem after confirming
- You want to undo the last change
- You need to quickly revert to a working state

## Transaction States

The transaction system has these states:

- **idle**: No transaction in progress
- **in_progress**: Currently applying changes
- **pending**: Waiting for confirmation
- **completed**: Successfully committed and confirmed
- **failed**: Transaction failed, rolled back

## Apply Commands (for Systemd)

These commands apply configurations without going through the staging/commit workflow. They're designed for systemd services to apply configs at boot.

### Network

```bash
# Apply current network configuration
hf network apply

# Bring down all managed interfaces
hf network down
```

### Firewall

```bash
# Apply current firewall rules
hf firewall apply

# Reload firewall rules (same as apply)
hf firewall reload

# Flush all firewall rules
hf firewall flush
```

### DHCP

```bash
# Apply current DHCP/DNS configuration
hf dhcp apply
```

## Systemd Integration

Services use the apply commands:

```ini
[Unit]
Description=Hellfire Network Configuration Service

[Service]
Type=oneshot
ExecStart=/usr/local/bin/hf network apply

[Install]
WantedBy=multi-user.target
```

```bash
# Restart network service
systemctl restart hellfire-network

# View service logs
journalctl -u hellfire-network -f
```

## Safety Features

### 1. Automatic Snapshots
Every commit creates a snapshot before applying changes. You can always go back.

### 2. Confirm-or-Revert
For critical changes (especially network), use `--confirm-timeout` to automatically rollback if you can't confirm.

### 3. Validation
Each applier validates that changes were applied correctly. If validation fails, the transaction rolls back.

### 4. Atomic Application
All configs are applied in order (network → firewall → dhcp). If any step fails, everything rolls back.

### 5. Event Bus
All transactions publish events, allowing other systems to react to configuration changes.

## Best Practices

### 1. Always Use Confirmation for Network Changes
```bash
hf commit -m "Network change" --confirm-timeout=60
```

### 2. Meaningful Commit Messages
```bash
hf commit -m "Add SSH firewall rule for remote access"
```

### 3. Test Before Confirming
After committing with a timeout:
1. Test your network connection
2. Verify services are running
3. Check firewall rules
4. Only then confirm

### 4. Keep Snapshots Clean
```bash
# Run periodically to keep only recent snapshots
hf snapshot prune --keep=30
```

### 5. Snapshot Before Major Changes
Even though commits auto-snapshot, you can manually snapshot:
```bash
# (Manual snapshot feature would go here if we add it)
```

## Examples

### Example 1: Safe Network Reconfiguration

```bash
# Current state
hf show network

# Stage changes
hf set network.wan.proto static
hf set network.wan.ipaddr 192.168.100.1
hf set network.wan.netmask 255.255.255.0
hf set network.wan.gateway 192.168.100.254

# Commit with 2-minute safety window
hf commit -m "Switch WAN to static IP" --confirm-timeout=120

# Test connection (SSH into router from another machine)
# Verify you can still access the router

# If OK, confirm
hf confirm

# If locked out, wait 2 minutes for auto-rollback
# Or, if you have console access:
hf rollback
```

### Example 2: Add Firewall Rule

```bash
# Add rule
hf set firewall.@rule[0].name "Allow-HTTPS"
hf set firewall.@rule[0].src wan
hf set firewall.@rule[0].proto tcp
hf set firewall.@rule[0].dest_port 443
hf set firewall.@rule[0].target ACCEPT

# Commit (firewall changes are less risky, no timeout needed)
hf commit -m "Add HTTPS firewall rule"
```

### Example 3: Disaster Recovery

```bash
# Something went wrong, need to go back
hf snapshot list

# Find the last known good snapshot
# Restore it
hf snapshot restore 20241006-120000

# Apply the restored configuration
hf commit -m "Emergency restore to last known good config"
```

## Comparison with Other Systems

| Feature | Hellfire | OpenWrt UCI | VyOS |
|---------|----------|-------------|------|
| Staging | ✓ | ✓ | ✓ |
| Snapshots | ✓ File-based | Manual | ✓ Boot images |
| Confirm-or-revert | ✓ | ✗ | ✓ |
| Atomic transactions | ✓ | Partial | ✓ |
| Point-in-time restore | ✓ | ✗ | ✓ |
| Auto-rollback | ✓ | ✗ | ✓ |

## Troubleshooting

### Transaction stuck in pending state

```bash
# Force rollback
hf rollback
```

### Can't confirm changes

```bash
# Check transaction state
# (Status command would be useful here)

# If timed out, it already rolled back
# Check logs for details
```

### Snapshot directory full

```bash
# Clean old snapshots
hf snapshot prune --keep=10
```

### Apply command fails

```bash
# Check service logs
journalctl -u hellfire-network -n 50

# Try applying manually
hf network apply

# If configuration is broken, restore a snapshot
hf snapshot list
hf snapshot restore <good-snapshot-id>
hf commit -m "Restore working config"
```

## Future Enhancements

- [ ] Diff between snapshots
- [ ] Named snapshots (tags)
- [ ] Scheduled snapshots
- [ ] Remote snapshot storage
- [ ] Transaction history/audit log
- [ ] Dry-run mode (preview changes without applying)
- [ ] Transaction status command
- [ ] Snapshot compression
