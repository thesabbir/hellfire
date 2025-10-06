# Build stage
FROM golang:1.25rc1-bookworm AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY cmd/ ./cmd/
COPY pkg/ ./pkg/

# Build the binary (with CGO for SQLite support)
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o hf ./cmd/hf

# Runtime stage
FROM debian:trixie-slim

# Prevent systemd from starting services during package installation
ENV DEBIAN_FRONTEND=noninteractive

# Install systemd and required packages
RUN apt-get update && apt-get install -y \
    systemd \
    systemd-sysv \
    nftables \
    iproute2 \
    iptables \
    net-tools \
    dnsmasq \
    curl \
    ca-certificates \
    procps \
    iputils-ping \
    dnsutils \
    vim \
    less \
    && rm -rf /var/lib/apt/lists/*

# Remove unnecessary systemd services for container
RUN systemctl mask \
    systemd-udevd.service \
    systemd-udevd-kernel.socket \
    systemd-udevd-control.socket \
    systemd-modules-load.service \
    sys-kernel-debug.mount \
    sys-kernel-tracing.mount \
    systemd-logind.service \
    getty.target \
    console-getty.service

# Create config and data directories
RUN mkdir -p /etc/config \
    && mkdir -p /var/lib/hellfire \
    && mkdir -p /var/lib/hellfire/audit-archive \
    && chmod 755 /var/lib/hellfire \
    && chmod 755 /var/lib/hellfire/audit-archive

# Copy the binary from builder
COPY --from=builder /build/hf /usr/local/bin/hf
RUN chmod +x /usr/local/bin/hf

# Copy example configs (including hellfire config)
COPY examples/config/* /etc/config/

# Copy systemd service files
COPY systemd/hellfire-api.service /etc/systemd/system/
COPY systemd/hellfire-network.service /etc/systemd/system/
COPY systemd/hellfire-firewall.service /etc/systemd/system/
COPY systemd/hellfire-dhcp.service /etc/systemd/system/

# Enable services
RUN systemctl enable hellfire-api.service \
    && systemctl enable hellfire-network.service \
    && systemctl enable hellfire-firewall.service \
    && systemctl enable hellfire-dhcp.service \
    && systemctl enable dnsmasq.service

# Expose API port
EXPOSE 8080

# Stop signal for systemd
STOPSIGNAL SIGRTMIN+3

# Run systemd as PID 1
CMD ["/lib/systemd/systemd"]
