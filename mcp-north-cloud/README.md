# MCP Index Manager Server

An MCP (Model Context Protocol) server that exposes tools for managing Elasticsearch indexes through the index-manager service.

## Overview

This MCP server provides a `delete_index` tool that allows you to delete Elasticsearch indexes via the MCP protocol. It communicates with the index-manager service's REST API to perform operations.

## Features

- **delete_index**: Delete an Elasticsearch index by name
  - Validates index name
  - Calls index-manager API
  - Returns success/error messages

## Architecture

The server implements the MCP protocol using:
- **stdio-based communication**: Reads from stdin, writes to stdout
- **JSON-RPC 2.0**: Standard MCP protocol format
- **HTTP client**: Communicates with index-manager service

## Usage

### Running Standalone

```bash
# Set the index-manager URL (defaults to http://localhost:8090)
export INDEX_MANAGER_URL=http://localhost:8090

# Run the server
./mcp-north-cloud
```

### Running with Docker

```bash
# Start the service
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d mcp-north-cloud

# View logs
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f mcp-north-cloud
```

### Cursor MCP Configuration

The project includes a `.cursor/mcp.json` file that configures Cursor to use this MCP server. The configuration uses Docker to run the server from the container.

**Project-specific configuration** (`.cursor/mcp.json`):
```json
{
  "mcpServers": {
    "index-manager": {
      "command": "docker",
      "args": [
        "exec",
        "-i",
        "north-cloud-mcp-north-cloud-1",
        "/app/tmp/mcp-north-cloud"
      ],
      "env": {
        "INDEX_MANAGER_URL": "http://index-manager:8090"
      }
    }
  }
}
```

**Global configuration** (for use outside this project):
- **macOS/Linux**: `~/.cursor/mcp.json`
- **Windows**: `%USERPROFILE%\.cursor\mcp.json`

After creating or modifying the configuration, **restart Cursor** to apply the changes.

### Alternative: Local Binary Configuration

If you prefer to run the MCP server locally instead of via Docker:

```json
{
  "mcpServers": {
    "index-manager": {
      "command": "/path/to/mcp-north-cloud",
      "env": {
        "INDEX_MANAGER_URL": "http://localhost:8090"
      }
    }
  }
}
```

## Available Tools

### delete_index

Deletes an Elasticsearch index by name.

**Parameters:**
- `index_name` (string, required): The name of the index to delete (e.g., `example_com_raw_content`)

**Example Request:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "delete_index",
    "arguments": {
      "index_name": "example_com_raw_content"
    }
  }
}
```

**Example Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Successfully deleted index: example_com_raw_content"
      }
    ],
    "isError": false
  }
}
```

## Development

### Prerequisites

- Go 1.25+
- Docker and Docker Compose (for containerized development)
- Access to index-manager service

### Building

```bash
go build -o mcp-north-cloud ./main.go
```

### Running Tests

```bash
go test ./...
```

### Hot Reloading (Development)

The service uses Air for hot reloading in development mode:

```bash
air -c .air.toml
```

## Environment Variables

- `INDEX_MANAGER_URL`: URL of the index-manager service (default: `http://localhost:8090`)

## Error Handling

The server returns standard JSON-RPC error responses:

- `-32700`: Parse error
- `-32600`: Invalid request
- `-32601`: Method not found
- `-32602`: Invalid params
- `-32603`: Internal error

## Protocol Support

- **Protocol Version**: 2024-11-05
- **Transport**: stdio (stdin/stdout)
- **Format**: JSON-RPC 2.0

## Security Considerations

- The server does not perform authentication - ensure it's only accessible to trusted clients
- Index deletion is irreversible - use with caution
- Consider adding authentication/authorization for production use

## Troubleshooting

### Server not responding

1. Check that index-manager service is running:
   ```bash
   curl http://localhost:8090/api/v1/health
   ```

2. Verify INDEX_MANAGER_URL environment variable is set correctly

3. Check server logs for errors

4. **For Cursor**: Ensure the container name matches in `.cursor/mcp.json`:
   ```bash
   docker ps | grep mcp-north-cloud
   ```
   Update the container name in the config if it differs.

### Index deletion fails

1. Verify the index exists:
   ```bash
   curl http://localhost:8090/api/v1/indexes
   ```

2. Check index-manager service logs:
   ```bash
   docker logs north-cloud-index-manager-1
   ```

3. Ensure the index name is correct (case-sensitive)

### Cursor not detecting MCP server

1. Verify `.cursor/mcp.json` exists in the project root
2. Check that the container is running:
   ```bash
   docker ps | grep mcp-north-cloud
   ```
3. Restart Cursor after modifying the configuration
4. Check Cursor's MCP server status in settings

## License

Part of the North Cloud project.
