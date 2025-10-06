#!/bin/bash
set -e

# This entrypoint script is optional for systemd containers
# It can be used for pre-start initialization if needed

# Create necessary directories
mkdir -p /run/systemd/system

# Set up cgroup v2 if needed
if [ ! -d /sys/fs/cgroup/systemd ]; then
    mkdir -p /sys/fs/cgroup/systemd
fi

# Execute systemd as PID 1
exec /lib/systemd/systemd
