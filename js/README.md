# @jtsang/nettune-mcp

MCP (Model Context Protocol) stdio wrapper for **nettune** - a TCP network optimization tool.

## Overview

This package provides an `npx`-compatible entry point for the nettune client. It automatically downloads the appropriate nettune binary for your platform and launches it in MCP stdio mode, enabling integration with Chat GUIs that support MCP.

## Installation

### Using npx (recommended)

No installation required - just run:

```bash
npx @jtsang/nettune-mcp --api-key YOUR_API_KEY --server http://your-server:9876
```

### Global Installation

```bash
npm install -g @jtsang/nettune-mcp
# or
bun install -g @jtsang/nettune-mcp
```

Then run:

```bash
nettune-mcp --api-key YOUR_API_KEY --server http://your-server:9876
```

## Usage

### Command Line Options

| Option | Required | Default | Description |
|--------|----------|---------|-------------|
| `--api-key`, `-k` | Yes | - | API key for server authentication |
| `--server`, `-s` | No | `http://127.0.0.1:9876` | Server URL |
| `--mcp-name` | No | `nettune` | MCP server name identifier |
| `--version-tag` | No | `latest` | Specific nettune version to use |
| `--verbose`, `-v` | No | `false` | Enable verbose logging |

### Example

```bash
# Connect to a remote server
npx @jtsang/nettune-mcp --api-key my-secret-key --server http://192.168.1.100:9876

# Use a specific version
npx @jtsang/nettune-mcp --api-key my-secret-key --version-tag v0.1.0

# Enable verbose logging
npx @jtsang/nettune-mcp --api-key my-secret-key --verbose
```

## Chat GUI Configuration

### Cursor / Windsurf / Claude Desktop

Add to your MCP configuration file:

```json
{
  "mcpServers": {
    "nettune": {
      "command": "npx",
      "args": [
        "@jtsang/nettune-mcp",
        "--api-key",
        "YOUR_API_KEY",
        "--server",
        "http://your-server:9876"
      ]
    }
  }
}
```

### Using with Bun

```json
{
  "mcpServers": {
    "nettune": {
      "command": "bunx",
      "args": [
        "@jtsang/nettune-mcp",
        "--api-key",
        "YOUR_API_KEY",
        "--server",
        "http://your-server:9876"
      ]
    }
  }
}
```

## Available MCP Tools

Once connected, the following tools are available:

| Tool | Description |
|------|-------------|
| `nettune.test_rtt` | Test RTT/latency to the server |
| `nettune.test_throughput` | Test download/upload throughput |
| `nettune.test_latency_under_load` | Test latency during load |
| `nettune.snapshot_server` | Create a server state snapshot |
| `nettune.list_profiles` | List available optimization profiles |
| `nettune.show_profile` | Show details of a specific profile |
| `nettune.create_profile` | Create a custom optimization profile |
| `nettune.apply_profile` | Apply an optimization profile |
| `nettune.rollback` | Rollback to a previous snapshot |
| `nettune.status` | Get current server status |

## Binary Caching

The wrapper automatically downloads and caches the nettune binary in:

- Linux: `$XDG_CACHE_HOME/nettune/` or `~/.cache/nettune/`
- macOS: `~/.cache/nettune/`
- Windows: `~/.cache/nettune/`

The binary is verified using SHA256 checksums when available.

## Supported Platforms

| OS | Architecture |
|----|--------------|
| Linux | x64, arm64 |
| macOS | x64, arm64 |
| Windows | x64, arm64 (experimental) |

## Server Setup

Before using this client, you need to start a nettune server:

```bash
# On the server (Linux)
sudo nettune server --api-key YOUR_API_KEY --listen 0.0.0.0:9876
```

## Security Notes

- The API key is passed via command line argument. Ensure your shell history is secured.
- All communication between client and server uses HTTP with Bearer token authentication.
- Consider using TLS termination (e.g., nginx reverse proxy) for production deployments.

## Troubleshooting

### Binary Download Fails

1. Check your internet connection
2. Verify the GitHub releases exist: https://github.com/jtsang4/nettune/releases
3. Try specifying a version: `--version-tag v0.1.0`

### Connection Refused

1. Verify the server is running
2. Check firewall rules allow traffic on port 9876
3. Ensure the server URL is correct (including `http://` prefix)

### Authentication Failed

1. Verify the API key matches between client and server
2. Check the server logs for authentication errors

## Development

```bash
# Install dependencies
bun install

# Build
bun run build

# Type check
bun run typecheck

# Test
bun test
```

## License

MIT
