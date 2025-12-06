# Nettune

A network diagnostics and TCP optimization tool with MCP (Model Context Protocol) integration for AI-assisted configuration.

## Features

- **End-to-end Network Testing**: RTT measurement, throughput testing, latency under load detection
- **Configuration Profiles**: Pre-built profiles for BBR, FQ, buffer tuning
- **Safe Apply/Rollback**: Automatic snapshots before configuration changes with rollback capability
- **MCP Integration**: Works with Claude and other LLM chat interfaces via MCP stdio protocol

## Quick Start

### Server Mode

Run on the Linux server you want to optimize:

```bash
# Start the server with an API key
sudo nettune server --api-key YOUR_SECRET_KEY

# With custom options
sudo nettune server --api-key YOUR_SECRET_KEY --listen 0.0.0.0:9876 --state-dir /var/lib/nettune
```

### Client Mode (MCP)

Configure in your Chat GUI (e.g., Claude Desktop):

```json
{
  "mcpServers": {
    "nettune": {
      "command": "nettune",
      "args": ["client", "--api-key", "YOUR_SECRET_KEY", "--server", "http://YOUR_SERVER:9876"]
    }
  }
}
```

Or use the NPM wrapper:

```json
{
  "mcpServers": {
    "nettune": {
      "command": "npx",
      "args": ["nettune-mcp", "--api-key", "YOUR_SECRET_KEY", "--server", "http://YOUR_SERVER:9876"]
    }
  }
}
```

## Available MCP Tools

| Tool | Description |
|------|-------------|
| `nettune.test_rtt` | Measure RTT (round-trip time) to server |
| `nettune.test_throughput` | Measure upload/download throughput |
| `nettune.test_latency_under_load` | Detect bufferbloat by measuring latency during load |
| `nettune.snapshot_server` | Create a configuration snapshot for rollback |
| `nettune.list_profiles` | List available optimization profiles |
| `nettune.show_profile` | Show details of a specific profile |
| `nettune.apply_profile` | Apply a profile (dry_run or commit mode) |
| `nettune.rollback` | Rollback to a previous snapshot |
| `nettune.status` | Get current server status and configuration |

## Built-in Profiles

### bbr-fq-default
Conservative BBR + FQ setup. Safe for most servers.

- Enables BBR congestion control
- Sets FQ as default qdisc
- Enables MTU probing

### bbr-fq-tuned-32mb
BBR with increased buffers for high-BDP (bandwidth-delay product) links.

- All settings from bbr-fq-default
- 32MB socket buffers (rmem_max, wmem_max)
- Optimized tcp_rmem/tcp_wmem
- Disables slow start after idle

## Building from Source

```bash
# Clone the repository
git clone https://github.com/jtsang4/nettune.git
cd nettune

# Install dependencies
go mod tidy

# Build
make build

# Build for all platforms
make build-all
```

## CLI Reference

### Server Command

```bash
nettune server [flags]

Flags:
  --api-key string      API key for authentication (required)
  --listen string       Address to listen on (default "0.0.0.0:9876")
  --state-dir string    Directory for state storage
  --read-timeout int    HTTP read timeout in seconds (default 30)
  --write-timeout int   HTTP write timeout in seconds (default 60)
```

### Client Command

```bash
nettune client [flags]

Flags:
  --api-key string    API key for authentication (required)
  --server string     Server URL (default "http://127.0.0.1:9876")
  --timeout int       Request timeout in seconds (default 60)
```

## HTTP API Endpoints

### Probe Endpoints
- `GET /probe/echo` - Latency test endpoint
- `GET /probe/download?bytes=N` - Download test
- `POST /probe/upload` - Upload test
- `GET /probe/info` - Server information

### Profile Endpoints
- `GET /profiles` - List profiles
- `GET /profiles/:id` - Get profile details

### System Endpoints
- `POST /sys/snapshot` - Create snapshot
- `GET /sys/snapshot/:id` - Get snapshot
- `POST /sys/apply` - Apply profile
- `POST /sys/rollback` - Rollback to snapshot
- `GET /sys/status` - Get system status

## Troubleshooting

### Permission Issues
The server needs root privileges to modify system settings:
```bash
sudo nettune server --api-key YOUR_KEY
```

### BBR Not Available
If BBR is not listed in available congestion controls, you may need to load the kernel module:
```bash
sudo modprobe tcp_bbr
```

### Firewall Issues
Ensure port 9876 (or your custom port) is accessible:
```bash
sudo ufw allow 9876/tcp
```

## Release Process

Releases are automated via GitHub Actions. When you push a version tag, the workflow will:

1. Build Go binaries for all platforms (linux/darwin × amd64/arm64)
2. Create a GitHub Release with binaries and checksums
3. Publish the NPM package to npm registry

### Creating a Release

```bash
# Tag a new version
git tag v0.1.0
git push origin v0.1.0
```

### Manual Testing

```bash
# Test Go build locally
make build-all

# Test JS build locally
cd js && bun install && bun run build && bun test
```

## License

MIT License

## Contributing

Contributions are welcome! Please read the contributing guidelines before submitting PRs.
