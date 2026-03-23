# IRCd Design Spec

## Overview

A minimal IRC server (IRCd) written in Go, living inside the north-cloud monorepo as `ircd/`. It implements enough of RFC 1459/2812 to work with any standard IRC client (irssi, WeeChat, HexChat) and the existing web client at `irc.northcloud.one`.

## Goals

- Run a personal/community IRC network at `irc.northcloud.one`
- Support standard IRC clients connecting and chatting
- Follow north-cloud conventions (Uber FX, Zap, Taskfile, testify)
- Keep the implementation simple and idiomatic Go

## Non-Goals (Future Iterations)

- Server-to-server linking (S2S)
- Services (NickServ/ChanServ)
- TLS
- Connection throttling / flood protection
- Channel bans (+b), invite-only (+i)
- IRCv3 capabilities negotiation
- State persistence across restarts

## Architecture

Goroutine-per-connection model. Each client gets a read goroutine and a write goroutine. A central `Server` struct holds shared state (clients map, channels map) protected by `sync.RWMutex`.

### Package Layout

```
north-cloud/ircd/
├── main.go                  # Entry point, Uber FX app bootstrap
├── go.mod                   # Module: github.com/jonesrussell/north-cloud/ircd
├── Taskfile.yml             # dev, build, lint, test tasks
├── config.yml.example       # Default config
├── cmd/
│   └── serve.go             # CLI command to start the server
├── internal/
│   ├── config/
│   │   └── config.go        # Config struct, loader
│   ├── server/
│   │   └── server.go        # TCP listener, accept loop, client registry
│   ├── client/
│   │   ├── client.go        # Client struct, read/write goroutines
│   │   └── message.go       # IRC message parsing
│   ├── channel/
│   │   └── channel.go       # Channel struct, membership, broadcast
│   └── command/
│       ├── handler.go       # Command dispatcher
│       ├── registration.go  # NICK, USER, PASS, QUIT
│       ├── messaging.go     # PRIVMSG, NOTICE
│       ├── channel.go       # JOIN, PART, TOPIC, NAMES, KICK, LIST
│       ├── query.go         # WHO, WHOIS, LUSERS, MOTD
│       ├── mode.go          # MODE (user + channel)
│       └── oper.go          # OPER, KILL
```

### Key Design Decisions

- **Uber FX** for dependency injection (north-cloud convention)
- **Zap** for structured logging (north-cloud convention)
- **Server** owns client map (`map[string]*Client`) and channel map (`map[string]*Channel`), protected by `sync.RWMutex`
- **Client** has two goroutines: `readLoop` (parses incoming lines, dispatches to command handlers) and `writeLoop` (drains an outbound `chan string`)
- **Command handlers** receive `(server, client, message)` and write responses directly to the client's send channel
- No dependency on Redis, Postgres, or Elasticsearch

## Data Flow

```
Client connects (TCP)
  → server.Accept() spawns client
  → client.readLoop() reads lines
  → each line parsed into Message{Prefix, Command, Params}
  → command.Dispatch(server, client, msg)
  → handler writes replies to client.Send channel
  → client.writeLoop() flushes Send channel to TCP conn
```

### Connection Lifecycle

1. Client connects → server assigns unique ID, adds to unregistered clients map
2. Client sends `NICK` + `USER` → server validates nick uniqueness, moves to registered map, sends welcome burst (RPL_WELCOME 001-005, MOTD)
3. Client sends commands → dispatched to handlers
4. Client sends `QUIT` or connection drops → server removes from all channels, notifies other users, cleans up

### Concurrency Model

- `server.clients` — `sync.RWMutex`, write-locked on connect/disconnect, read-locked for lookups
- `server.channels` — `sync.RWMutex`, write-locked on channel create/destroy, read-locked for lookups
- `channel.members` — `sync.RWMutex` per channel, write-locked on join/part, read-locked for broadcast
- `client.Send` — buffered `chan string` (size ~512), writeLoop drains it; if buffer fills (slow client), connection is killed

### PING/PONG Keepalive

Server sends `PING :servername` every 90 seconds. If no `PONG` within 120 seconds, connection is closed.

## IRC Protocol Support

### Message Format

```
[:prefix] COMMAND [params] [:trailing]
\r\n terminated, max 512 bytes per line
```

### Commands Implemented

| Category | Commands |
|----------|----------|
| Registration | NICK, USER, PASS, QUIT |
| Messaging | PRIVMSG, NOTICE |
| Channels | JOIN, PART, TOPIC, NAMES, KICK, LIST |
| Queries | WHO, WHOIS, LUSERS, MOTD |
| Modes | MODE (user + channel) |
| Operator | OPER, KILL |
| Keepalive | PING, PONG |

### Channel Modes

| Mode | Meaning |
|------|---------|
| +o | Channel operator |
| +t | Topic lock (ops only) |
| +n | No external messages |

### User Modes

| Mode | Meaning |
|------|---------|
| +i | Invisible |
| +o | IRC operator |

### Numeric Replies

| Code | Name | When |
|------|------|------|
| 001-005 | Welcome burst | After NICK+USER |
| 331/332 | Topic replies | JOIN, TOPIC query |
| 353/366 | Names list | JOIN, NAMES |
| 375/372/376 | MOTD | On connect, MOTD command |
| 401 | No such nick | PRIVMSG to unknown user |
| 403 | No such channel | Operations on nonexistent channel |
| 431/432/433 | Nick errors | Empty, invalid, already in use |
| 461 | Need more params | Missing required params |
| 462 | Already registered | Double USER command |

## Configuration

```yaml
server:
  name: irc.northcloud.one
  network: NorthCloud
  listen: ":6667"
  max_clients: 256
  ping_interval: 90s
  pong_timeout: 120s
  motd: |
    Welcome to NorthCloud IRC.
    This server is part of the NorthCloud network.

opers:
  - name: jones
    password: "$2a$10$..."  # bcrypt hash
```

## Integration with North Cloud

- Added to `go.work` as a workspace member
- Uses `infrastructure/` packages where appropriate (config loading, structured logging)
- Own `Taskfile.yml` with standard `dev`, `build`, `lint`, `test` tasks
- Runs standalone — no dependency on other north-cloud services
- Default port 6667 (standard IRC plaintext)

## Testing Strategy

### Unit Tests

- **Message parsing** — well-formed, malformed, and edge-case IRC lines
- **Command handlers** — inject mock server/client, assert replies on Send channel
- **Channel operations** — join/part/broadcast with multiple mock clients
- **Nick validation** — RFC rules (no spaces, can't start with digit, max length)

### Integration Tests

- Spin up server on random port, connect real TCP clients, exchange messages
- Registration flow (NICK + USER → welcome burst)
- Multi-client scenarios (two clients in channel, message delivery)
- Error cases (duplicate nick, unknown command, nonexistent channel)

### Tools

- `testify` for assertions (north-cloud convention)
- `net.Pipe()` or localhost TCP for integration tests
- Real connections, no TCP mocking
