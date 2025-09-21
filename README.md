# Web MCP

A Model Context Protocol (MCP) server that provides web search and content
fetching capabilities.

## Features

- **Web Search**: Search the web and get formatted results
- **Web Fetch**: Fetch and parse web page content
- **Caching**: Built-in caching for improved performance
- **HTTPS Upgrade**: Automatically upgrades HTTP URLs to HTTPS

## Tools

### `web-search`

Search the web for current information and recent data.

**Parameters:**

- `query` (required): The search query

### `web-fetch`

Fetch content from a URL and return the parsed content.

**Parameters:**

- `url` (required): The URL to fetch

## Installation

```bash
go build -o web-mcp ./cmd/server
```

## Usage

Run the server:

```bash
./web-mcp
```

## MCP Configuration

To use this server with Claude Desktop or other MCP clients, add it to your MCP
configuration file.

```json
{
  "mcpServers": {
    "web": {
      "command": "/path/to/web-mcp",
      "args": []
    }
  }
}
```

Replace `/path/to/web-mcp` with the actual path to your compiled binary.

### Other MCP Clients

For other MCP clients, configure the server with:

- **Command**: Path to the `web-mcp` binary
- **Args**: Empty array `[]`
- **Transport**: STDIO

## Configuration

Set the cache path with the `WEB_MCP_CACHE` environment variable (defaults to
`./.cache.bbolt`).

## Requirements

- Go 1.25.1+

## License

MIT
