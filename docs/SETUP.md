# Arqut Edge Setup Guide (Community Edition)

Complete guide to setting up and running Arqut Edge devices on your local network. Edge devices provide WireGuard VPN connectivity and integrate with the Arqut Server for peer discovery and signaling.

> **Note:** This guide assumes you have already installed the [Arqut Server](SERVER_SETUP.md) and have your API key ready.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Installation](#installation)
3. [Configuration](#configuration)
4. [Running the Edge Service](#running-the-edge-service)
5. [Testing Your Setup](#testing-your-setup)
6. [Production Deployment](#production-deployment)
7. [Troubleshooting](#troubleshooting)

## Prerequisites

### Hardware Requirements

- **Device**: PC, Raspberry Pi, NAS, or any Linux-capable device
- **CPU**: 1+ cores
- **RAM**: 512MB minimum (1GB+ recommended)
- **Storage**: 1GB minimum
- **Network**: Ethernet or WiFi connection to local network

### Software Requirements

- **OS**: Ubuntu, Debian, or Raspberry Pi OS
- **Go**: Version 1.24 or later (for building from source)
- **systemd**: For service management (included in most modern distros)

### From Server Setup

You must have completed the [Server Setup](SERVER_SETUP.md) and have:

- ✅ **API Key** generated from server (starts with `arq_`)
- ✅ **Server URL** (e.g., `https://yourdomain.com:9000`)
- ✅ Server accessible from the internet

### Network Access

- **Outbound HTTPS**: Edge must reach the server
- **Local network access**: For serving VPN clients
- **Port 3030**: Default API port (configurable)

## Installation

### Supported Architectures

Arqut Edge supports:

- `x86_64` (64-bit Intel/AMD)
- `arm64` (64-bit ARM - Raspberry Pi 4/5)
- `armv7` (32-bit ARM - Raspberry Pi 2/3)
- `armv6` (Legacy ARM - Raspberry Pi 1/Zero)
- `i386` (32-bit Intel/AMD)

### Option 1: Build from Source (Recommended)

```bash
# Clone the repository
git clone https://github.com/arqut/arqut-edge-ce.git
cd arqut-edge-ce

# Build the binary
make build

# Binary will be available at ./bin/arqut-edge-ce-app
```

### Option 2: Download Pre-built Binary

```bash
# Detect your architecture
ARCH=$(uname -m)
case $ARCH in
  x86_64)   SUFFIX="x86_64" ;;
  aarch64)  SUFFIX="arm64" ;;
  armv7l)   SUFFIX="armv7" ;;
  armv6l)   SUFFIX="armv6" ;;
  i686)     SUFFIX="i386" ;;
  *)        echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Download binary
wget "https://www.arqut.com/downloads/arqut-edge/releases/latest/arqut_edge_latest_Linux_${SUFFIX}.tar.gz"

# Extract
tar -xzf "arqut_edge_latest_Linux_${SUFFIX}.tar.gz"
chmod +x arqut-edge

# Move to system location
sudo mv arqut-edge /usr/local/bin/arqut-edge-ce
```

### Option 3: Automated Install Script (Ubuntu/Debian/Raspberry Pi)

For a quick automated setup:

```bash
# Download and run install script
curl -fsSL https://www.arqut.com/downloads/arqut-edge/install.sh | sudo bash
```

This script will:

- Detect your OS and architecture
- Download and install binaries
- Create systemd services
- Set up directories with proper permissions

## Configuration

### Step 1: Create Installation Directory

```bash
# Create app directory
sudo mkdir -p /opt/arqut-edge
sudo mkdir -p /opt/arqut-edge/data

# Create user for running the service
sudo useradd -r -s /bin/false arqut-edge

# Set ownership
sudo chown -R arqut-edge:arqut-edge /opt/arqut-edge
```

### Step 2: Create Environment File

Create the environment configuration file:

```bash
sudo nano /opt/arqut-edge/arqut.env
```

Add your configuration (replace with your actual values):

```bash
# REQUIRED: API key from server setup
ARQUT_API_KEY=arq_your_api_key_from_server_setup

# REQUIRED: Server WebSocket URL
CLOUD_URL=wss://yourdomain.com:9000

# OPTIONAL: Edge API listen address
SERVER_ADDR=:3030

# OPTIONAL: Database path
DB_PATH=/opt/arqut-edge/data/edge.db

# OPTIONAL: Log level (debug, info, warn, error)
LOG_LEVEL=info
```

**Important Configuration Notes:**

| Variable        | Description              | Example                     | Required                       |
| --------------- | ------------------------ | --------------------------- | ------------------------------ |
| `ARQUT_API_KEY` | API key from server      | `arq_abc123...`             | ✅ Yes                         |
| `CLOUD_URL`     | Server WebSocket URL     | `wss://yourdomain.com:9000` | ✅ Yes                         |
| `SERVER_ADDR`   | Local API listen address | `:3030`                     | No (default: `:3030`)          |
| `DB_PATH`       | SQLite database path     | `./data/edge.db`            | No (default: `./data/edge.db`) |
| `LOG_LEVEL`     | Logging verbosity        | `info`                      | No (default: `info`)           |

**WSL Users:** If you encounter SQLite "out of memory (14)" errors on WSL, use:

```bash
DB_PATH=/tmp/arqut-edge.db
```

This is a WSL-specific issue with file locking on Windows filesystem paths.

### Step 3: Secure the Environment File

```bash
# Set restrictive permissions on sensitive files
sudo chmod 600 /opt/arqut-edge/arqut.env
sudo chown arqut-edge:arqut-edge /opt/arqut-edge/arqut.env
```

**Why 600 permissions?** The environment file contains your API key, which is sensitive. Only the service user should be able to read it.

## Running the Edge Service

### Option 1: Development Mode (Testing)

For testing and development:

```bash
# Run directly
cd arqut-edge-ce
./bin/arqut-edge-ce-app
```

Press `Ctrl+C` to stop.

### Option 2: Production Mode with Systemd (Recommended)

#### 1. Create Systemd Service File

```bash
sudo nano /etc/systemd/system/arqut-edge.service
```

Add the following configuration:

```ini
[Unit]
Description=Arqut Edge Service
After=network.target docker.service
Wants=docker.service

[Service]
Type=simple
User=arqut-edge
Group=arqut-edge
SupplementaryGroups=docker

# Load environment variables
EnvironmentFile=/opt/arqut-edge/arqut.env

# Binary location
ExecStart=/usr/local/bin/arqut-edge-ce
WorkingDirectory=/opt/arqut-edge

# Security settings
NoNewPrivileges=false
CapabilityBoundingSet=CAP_NET_ADMIN
AmbientCapabilities=CAP_NET_ADMIN

# Restart policy
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

**Security Configuration Explained:**

- `User=arqut-edge` - Run as dedicated non-root user (least privilege)
- `SupplementaryGroups=docker` - Access to Docker socket (if Docker features needed)
- `CapabilityBoundingSet=CAP_NET_ADMIN` - Allows network configuration (WireGuard)
- `AmbientCapabilities=CAP_NET_ADMIN` - Grants network admin capability at runtime
- `NoNewPrivileges=false` - Required for capability inheritance

#### 2. Enable and Start Service

```bash
# Reload systemd configuration
sudo systemctl daemon-reload

# Enable service to start on boot
sudo systemctl enable arqut-edge

# Start the service
sudo systemctl start arqut-edge

# Check status
sudo systemctl status arqut-edge
```

#### 3. View Logs

```bash
# Follow logs in real-time
sudo journalctl -u arqut-edge -f

# View recent logs
sudo journalctl -u arqut-edge -n 100

# View logs since boot
sudo journalctl -u arqut-edge -b

# Filter by log level
sudo journalctl -u arqut-edge -p err  # Errors only
```

### Service Management Commands

```bash
# Start service
sudo systemctl start arqut-edge

# Stop service
sudo systemctl stop arqut-edge

# Restart service
sudo systemctl restart arqut-edge

# Check status
sudo systemctl status arqut-edge

# Disable auto-start
sudo systemctl disable arqut-edge

# Enable auto-start
sudo systemctl enable arqut-edge
```

## Testing Your Setup

### 1. Check Edge Health

```bash
# Health check endpoint
curl http://localhost:3030/api/health
```

Expected response:

```json
{
  "status": "ok",
  "version": "1.0.0"
}
```

### 2. Verify Server Connection

Check logs for successful connection to server:

```bash
sudo journalctl -u arqut-edge -n 50 | grep -i "signaling\|connected"
```

You should see messages like:

```
[Signaling] Connected to server: wss://yourdomain.com:9000
[WireGuard/Manager] Register with signaling server...
[WireGuard/Manager] Requesting TURN credentials...
```

### 3. Check WireGuard Status

```bash
# List WireGuard interfaces
curl http://localhost:3030/api/wireguard/interfaces
```

Expected response (initially empty):

```json
{
  "data": {
    "interfaces": {}
  }
}
```

### 4. Check Proxy Services

```bash
# List configured proxy services
curl http://localhost:3030/api/proxy/services
```

### 5. Monitor Logs for Errors

```bash
# Watch for any errors
sudo journalctl -u arqut-edge -f | grep -i "error\|fail\|warn"
```

## Production Deployment

### Firewall Configuration

If you have a firewall enabled (UFW, firewalld, etc.):

```bash
# Allow edge API port
sudo ufw allow 3030/tcp

# Or for firewalld
sudo firewall-cmd --permanent --add-port=3030/tcp
sudo firewall-cmd --reload
```

### Automatic Updates (Optional)

Create an updater service (if using pre-built binaries):

```bash
# Download updater binary
sudo wget -O /usr/local/bin/arqut-edge-updater \
  "https://www.arqut.com/downloads/arqut-edge/updater/releases/latest/arqut_edge_updater_latest_Linux_${SUFFIX}.tar.gz"

sudo chmod +x /usr/local/bin/arqut-edge-updater

# Create updater service
sudo nano /etc/systemd/system/arqut-edge-updater.service
```

```ini
[Unit]
Description=Arqut Edge Updater Service
After=network.target

[Service]
Type=simple
User=arqut-edge
Group=arqut-edge
ExecStart=/usr/local/bin/arqut-edge-updater
WorkingDirectory=/opt/arqut-edge
Restart=always

[Install]
WantedBy=multi-user.target
```

```bash
# Enable updater
sudo systemctl daemon-reload
sudo systemctl enable arqut-edge-updater
sudo systemctl start arqut-edge-updater
```

### Log Rotation

Set up log rotation to prevent disk space issues:

```bash
sudo nano /etc/logrotate.d/arqut-edge
```

```
/var/log/arqut-edge/*.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
    create 0640 arqut-edge arqut-edge
    sharedscripts
    postrotate
        systemctl reload arqut-edge
    endscript
}
```

### Monitoring

Monitor key metrics:

```bash
# CPU and memory usage
systemctl status arqut-edge

# Network connections
sudo netstat -tunlp | grep arqut-edge

# Disk usage
du -sh /opt/arqut-edge
```

## Troubleshooting

### Service Won't Start

**Check logs:**

```bash
sudo journalctl -u arqut-edge -n 50
```

**Common issues:**

1. **Missing API key:**

   ```
   ERROR: ARQUT_API_KEY environment variable not set
   ```

   Solution: Check `/opt/arqut-edge/arqut.env` has valid API key

2. **Can't connect to server:**

   ```
   ERROR: Failed to connect to signaling server
   ```

   Solution: Verify `CLOUD_URL` is correct and server is reachable:

   ```bash
   curl https://yourdomain.com:9000/api/v1/health
   ```

3. **Port already in use:**

   ```
   ERROR: listen tcp :3030: bind: address already in use
   ```

   Solution: Change `SERVER_ADDR` in `arqut.env` or stop conflicting service:

   ```bash
   sudo lsof -i :3030
   ```

4. **Permission denied:**

   ```
   ERROR: permission denied: /opt/arqut-edge/data/edge.db
   ```

   Solution:

   ```bash
   sudo chown -R arqut-edge:arqut-edge /opt/arqut-edge
   ```

5. **SQLite out of memory (WSL users):**
   ```
   ERROR: database disk image is malformed (14)
   ```
   Solution: Use `/tmp` for database on WSL:
   ```bash
   # Edit /opt/arqut-edge/arqut.env
   DB_PATH=/tmp/arqut-edge.db
   ```

### Can't Connect to Server

**Verify network connectivity:**

```bash
# Check server is reachable
curl https://yourdomain.com:9000/api/v1/health

# Check WebSocket connectivity
npm install -g wscat
wscat -c "wss://yourdomain.com:9000/api/v1/signaling/ws/edge?id=test" \
  -H "Authorization: Bearer arq_your_api_key"
```

**Check firewall:**

```bash
# Ensure outbound HTTPS is allowed
sudo ufw status
```

### WireGuard Not Working

**Check kernel modules:**

```bash
# Verify WireGuard module is loaded
lsmod | grep wireguard

# Load if missing
sudo modprobe wireguard
```

**Check logs:**

```bash
sudo journalctl -u arqut-edge | grep -i "wireguard\|wg"
```

### High CPU/Memory Usage

```bash
# Check resource usage
top -p $(pgrep arqut-edge)

# Check for memory leaks
sudo journalctl -u arqut-edge | grep -i "memory\|oom"

# Restart if needed
sudo systemctl restart arqut-edge
```

### Database Issues

**Reset database (WARNING: deletes all data):**

```bash
# Stop service
sudo systemctl stop arqut-edge

# Backup current database
sudo cp /opt/arqut-edge/data/edge.db /opt/arqut-edge/data/edge.db.backup

# Remove database
sudo rm /opt/arqut-edge/data/edge.db

# Restart service (creates new database)
sudo systemctl start arqut-edge
```

## Uninstallation

If you need to remove Arqut Edge:

```bash
# Stop and disable service
sudo systemctl stop arqut-edge
sudo systemctl disable arqut-edge

# Remove service files
sudo rm /etc/systemd/system/arqut-edge.service
sudo systemctl daemon-reload

# Remove application files
sudo rm -rf /opt/arqut-edge
sudo rm /usr/local/bin/arqut-edge-ce

# Remove user (optional)
sudo userdel arqut-edge

# Remove updater (if installed)
sudo systemctl stop arqut-edge-updater
sudo systemctl disable arqut-edge-updater
sudo rm /etc/systemd/system/arqut-edge-updater.service
sudo rm /usr/local/bin/arqut-edge-updater
```

## Architecture Overview

For a detailed understanding of how edge devices work:

```
┌──────────────────────────────────────┐
│         Arqut Edge Service           │
├──────────────────────────────────────┤
│                                      │
│  ┌────────────────────────────────┐  │
│  │   Service Registry             │  │
│  │  (Lifecycle Management)        │  │
│  └───────────┬────────────────────┘  │
│              │                       │
│      ┌───────────────────────┐       │
│      │                       │       │
│      ▼                       ▼       │
│ ┌──────────┐    ┌──────────────────┐ │
│ │WireGuard │    │ Proxy Provider   │ │
│ │ Manager  │    │ (HTTP/TCP/UDP)   │ │
│ └────┬─────┘    └────┬─────────────┘ │
│      │               ▼               │
│      │  ┌───────────────────────┐    │
│      └─►│ Signaling Client      │    │
│         │ (WebSocket to Server) │    │
│         └───────────────────────┘    │
│                                      │
└──────────────────────────────────────┘
```

### Service Components

1. **Service Registry** - Manages lifecycle of all services (Initialize → Start → Stop)
2. **WireGuard Manager** - Handles VPN connections and peer management
3. **Proxy Provider** - Manages HTTP/TCP/UDP proxying to local services
4. **Signaling Client** - Maintains WebSocket connection to server for signaling

## Next Steps

After successful edge installation:

1. **Configure client devices** to connect to your edge VPN
2. **Add proxy services** for local network access
3. **Monitor logs** regularly for errors
4. **Set up backups** of configuration
5. **Enable auto-updates** for maintenance

## Getting Help

- **Server Issues**: See [Server Setup Guide](SERVER_SETUP.md)
- **Installation Overview**: See [Installation Guide](INSTALLATION_GUIDE.md)
- **Logs**: Always check `sudo journalctl -u arqut-edge -f` first
- **GitHub Issues**: Report bugs and request features

---

**Edge setup complete!** Your edge device is now connected to the server and ready to serve VPN clients.
