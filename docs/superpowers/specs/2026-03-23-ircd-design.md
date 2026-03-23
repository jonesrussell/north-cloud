# IRCd Design Spec

## Overview

A minimal IRC server (IRCd) written in Go, living inside the north-cloud monorepo as `ircd/`. It implements enough of RFC 1459/2812 to work with any standard IRC client (irssi, WeeChat, HexChat) and the existing web client at `irc.northcloud.one`.

## Goals

- Run a personal/community IRC network at `irc.northcloud.one`
- Support standard IRC clients connecting and chatting
- Follow north-cloud conventions (constructor-based DI, Zap via infrastructure, Taskfile, testify)
- Keep the implementation simple and idiomatic Go

## Non-Goals (Future Iterations)

- Server-to-server linking (S2S)
- Services (NickServ/ChanServ)
- Native TLS (Caddy handles TLS termination — see Network Security below)
- Connection throttling / flood protection
- Channel bans (+b), invite-only (+i)
- IRCv3 capabilities negotiation
- State persistence across restarts
- WebSocket gateway for the web client (future iteration)

## Architecture

Goroutine-per-connection model. Each client gets a read goroutine and a write goroutine. A central `Server` struct holds shared state (clients map, channels map) protected by `sync.RWMutex`.

### Package Layout

```
north-cloud/ircd/
├── main.go                  # Entry point, constructor-based bootstrap
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

- **Constructor-based DI** (north-cloud convention — no framework, manual wiring)
- **Zap** for structured logging via `infrastructure/logger` (north-cloud convention)
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
2. Client optionally sends `PASS` → server stores the password for later OPER use (PASS is not used for connection authentication in MVP — the server is open)
3. Client sends `NICK` + `USER` → server validates nick uniqueness, moves to registered map, sends welcome burst (RPL_WELCOME 001-005, MOTD)
4. Client sends commands → dispatched to handlers
5. Client sends `QUIT` or connection drops → server removes from all channels, notifies other users, cleans up

### Graceful Shutdown

On SIGTERM/SIGINT (via `os/signal`):
1. Stop accepting new connections
2. Send `ERROR :Server shutting down` to all connected clients
3. Close all client connections
4. Exit cleanly

### Concurrency Model

- `server.clients` — `sync.RWMutex`, write-locked on connect/disconnect, read-locked for lookups
- `server.channels` — `sync.RWMutex`, write-locked on channel create/destroy, read-locked for lookups
- `channel.members` — `sync.RWMutex` per channel, write-locked on join/part, read-locked for broadcast
- `client.Send` — buffered `chan string` (size 512), writeLoop drains it; if buffer fills (slow client), server sends `ERROR :SendQ exceeded` and closes the connection (logged at WARN level)

### PING/PONG Keepalive

Server sends `PING :servername` every 90 seconds. If no `PONG` within 120 seconds, connection is closed.

## IRC Protocol Support

### Message Format

```
[:prefix] COMMAND [params] [:trailing]
\r\n terminated, max 512 bytes per line
```

### Commands Implemented

**Phase 1 (this plan):**

| Category | Commands |
|----------|----------|
| Registration | NICK, USER, PASS, QUIT |
| Messaging | PRIVMSG, NOTICE |
| Channels | JOIN, PART, TOPIC, NAMES, LIST |
| Keepalive | PING, PONG |

**Phase 2 (follow-up plan):**

| Category | Commands |
|----------|----------|
| Channels | KICK |
| Queries | WHO, WHOIS |
| Modes | MODE (user + channel) |
| Operator | OPER, KILL |

Note: LUSERS and MOTD are sent as part of the welcome burst in Phase 1 but not as standalone commands until Phase 2.

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
| 311/312/318 | WHOIS replies | WHOIS query |
| 352/315 | WHO replies | WHO query |
| 321/322/323 | LIST replies | LIST query |
| 482 | Chan op needed | KICK/TOPIC/MODE without +o |

## Error Handling

- **Lines exceeding 512 bytes**: truncated at 512 bytes (including `\r\n`), remainder discarded
- **Malformed messages**: silently ignored (standard IRCd behavior) with DEBUG-level log
- **Invalid UTF-8**: passed through as-is (IRC is historically encoding-agnostic)
- **TCP errors in readLoop**: treated as disconnect, triggers cleanup
- **Unknown commands**: reply with `421 ERR_UNKNOWNCOMMAND`

## Network Security

The IRCd listens on plaintext TCP port 6667 on localhost only. **Caddy** (already used across north-cloud deployments) handles TLS termination and proxies TCP to the IRCd. This means:

- Clients connect to `irc.northcloud.one:6697` (TLS) → Caddy → `127.0.0.1:6667` (plaintext)
- OPER passwords are protected in transit by Caddy's TLS
- No need to implement TLS in the IRCd itself for MVP

## Configuration

```yaml
server:
  name: irc.northcloud.one
  network: NorthCloud
  listen: "127.0.0.1:6667"
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

- Added to `go.work` as a workspace member (own `go.mod`, separate module)
- May import from `infrastructure/` only if `infrastructure/` is also a workspace module (verify at implementation time — if not, vendor any needed utilities locally)
- Own `Taskfile.yml` with standard `dev`, `build`, `lint`, `test` tasks
- Runs standalone — no dependency on other north-cloud services
- Default port 6667 (standard IRC plaintext, localhost only)
- The existing web client at `irc.northcloud.one` will not connect directly in MVP — a WebSocket gateway is a future iteration

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
