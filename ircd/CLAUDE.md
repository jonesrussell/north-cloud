# CLAUDE.md — IRCd

## Overview

Minimal IRC server (IRCd) written in Go, part of the north-cloud monorepo. Implements core RFC 1459/2812 commands for a personal/community IRC network.

## Architecture

Goroutine-per-connection model. Each client gets read/write goroutines. Central `Server` struct holds shared state protected by `sync.RWMutex`.

```
internal/
├── config/     Config loading (YAML + defaults + validation)
├── message/    IRC message parsing and formatting
├── client/     Client struct, read/write loops, send channel
├── channel/    Channel struct, membership, broadcast
├── server/     TCP listener, client/channel registry, command dispatch
└── command/    IRC command handlers (NICK, JOIN, PRIVMSG, etc.)
```

## Commands

```bash
task build          # Build to ./bin/ircd
task test           # Run all tests
task test:race      # Tests with race detector
task lint           # Format + vet
task run            # Run the server
```

## Configuration

Copy `config.yml.example` to `config.yml` and edit. Environment override: `IRCD_CONFIG=path/to/config.yml`.

## Key Conventions

- Constructor-based DI (no framework)
- Zap logging via `infrastructure/logger`
- testify for assertions
- `command.ServerInterface` decouples handlers from server implementation
