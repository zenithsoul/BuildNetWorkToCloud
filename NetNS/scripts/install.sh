#!/bin/bash
#
# install.sh - Install netns-mgr and systemd service
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

echo "Installing netns-mgr..."

# Check root
if [[ $EUID -ne 0 ]]; then
    echo "Please run as root: sudo $0"
    exit 1
fi

# Build the Go binary (if go is available)
if command -v go &>/dev/null; then
    echo "Building netns-mgr..."
    cd "$PROJECT_DIR"
    go build -o netns-mgr ./cmd/netns-mgr
    cp netns-mgr /usr/local/bin/
    echo "Installed: /usr/local/bin/netns-mgr"
fi

# Install restore script
cp "$SCRIPT_DIR/netns-restore.sh" /usr/local/bin/
chmod +x /usr/local/bin/netns-restore.sh
echo "Installed: /usr/local/bin/netns-restore.sh"

# Install systemd service
cp "$SCRIPT_DIR/netns-mgr.service" /etc/systemd/system/
systemctl daemon-reload
echo "Installed: /etc/systemd/system/netns-mgr.service"

# Enable service
systemctl enable netns-mgr.service
echo "Enabled netns-mgr.service"

# Create log file
touch /var/log/netns-restore.log
chmod 644 /var/log/netns-restore.log

echo ""
echo "Installation complete!"
echo ""
echo "Commands:"
echo "  systemctl start netns-mgr    # Restore namespaces now"
echo "  systemctl status netns-mgr   # Check status"
echo "  systemctl stop netns-mgr     # Cleanup namespaces"
echo "  journalctl -u netns-mgr      # View logs"
echo ""
echo "The service will automatically restore namespaces on boot."
