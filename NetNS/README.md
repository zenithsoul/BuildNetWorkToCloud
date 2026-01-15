# NetNS Manager

A Go-based tool for managing Linux network namespaces, virtual ethernet pairs, bridges, GRE tunnels, and routing.

## Overview

This project is an example implementation for building a **VPC (Virtual Private Cloud)** using Linux network namespaces. It demonstrates how cloud providers create isolated network environments for their customers.

By leveraging Linux network namespaces, this tool simulates the core networking concepts found in cloud platforms like AWS VPC, Google Cloud VPC, or Azure VNet:

- **Network Isolation** - Each namespace acts as an isolated network environment (like a VPC)
- **Virtual Networks** - Veth pairs connect isolated environments (like VPC peering)
- **Inter-host Connectivity** - GRE tunnels enable communication across physical hosts (like cross-region connectivity)
- **Software-Defined Networking** - Bridges and routes create flexible network topologies

## Features

- **Namespace Management** - Create, delete, and list network namespaces
- **Veth Pairs** - Create virtual ethernet pairs between namespaces
- **Bridge** - Configure Linux bridges
- **GRE Tunnels** - Set up GRE tunnels between hosts
- **IP Configuration** - Assign IP addresses to interfaces
- **Routing** - Configure routes within namespaces
- **REST API** - HTTP API server for remote management
- **SQLite Database** - Persistent storage for configurations

## Requirements

- Linux (network namespaces are Linux-specific)
- Go 1.24+
- Root privileges (required for network namespace operations)

## Installation

```bash
# Build
go build -o netns-mgr ./cmd/netns-mgr

# Install (optional)
./scripts/install.sh
```

## Usage

```bash
# Namespace commands
netns-mgr namespace create <name>
netns-mgr namespace delete <name>
netns-mgr namespace list

# Veth commands
netns-mgr veth create <name> --peer <peer-name>

# Bridge commands
netns-mgr bridge create <name>

# GRE tunnel commands
netns-mgr gre create <name> --local <ip> --remote <ip>

# IP commands
netns-mgr ip add <address> --dev <interface>

# Route commands
netns-mgr route add <destination> --via <gateway>

# Start API server
netns-mgr server
```

## Configuration

See `scripts/netns-config.yaml.example` for configuration options.

## Systemd Service

```bash
# Install as systemd service
sudo cp scripts/netns-mgr.service /etc/systemd/system/
sudo systemctl enable netns-mgr
sudo systemctl start netns-mgr
```

## Project Structure

```
NetNS/
├── cmd/netns-mgr/     # Main entry point
├── internal/
│   ├── api/           # REST API handlers
│   ├── cli/           # CLI commands (Cobra)
│   ├── config/        # Configuration
│   ├── db/            # SQLite database
│   └── netns/         # Network namespace operations
└── scripts/           # Installation and restore scripts
```

## License

MIT
