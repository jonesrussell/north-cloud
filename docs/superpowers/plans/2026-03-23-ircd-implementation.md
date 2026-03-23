# IRCd Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a minimal IRC server in Go that supports standard IRC clients (NICK, JOIN, PRIVMSG, etc.) inside the north-cloud monorepo.

**Architecture:** Goroutine-per-connection model with a central Server struct holding shared state (clients, channels) protected by RWMutex. Each client has a read goroutine (parses IRC lines, dispatches commands) and a write goroutine (drains a buffered send channel to the TCP connection).

**Tech Stack:** Go 1.26.1, north-cloud infrastructure (logger, config), testify, Taskfile

**Spec:** `docs/superpowers/specs/2026-03-23-ircd-design.md`

---

## File Structure

```
ircd/
├── main.go                          # Entry point, signal handling, bootstrap
├── go.mod                           # Module: github.com/jonesrussell/north-cloud/ircd
├── Taskfile.yml                     # Standard tasks: build, test, lint, dev
├── config.yml.example               # Default configuration
├── internal/
│   ├── config/
│   │   └── config.go                # Config struct, Load(), SetDefaults(), Validate()
│   ├── message/
│   │   └── message.go               # IRC message parsing and formatting
│   ├── client/
│   │   └── client.go                # Client struct, readLoop, writeLoop, Send channel
│   ├── channel/
│   │   └── channel.go               # Channel struct, membership, broadcast
│   ├── server/
│   │   └── server.go                # TCP listener, accept loop, client/channel registry, dispatch
│   └── command/
│       ├── handler.go               # HandlerFunc type, command registry, dispatcher
│       ├── registration.go          # NICK, USER, PASS, QUIT, PING, PONG
│       ├── messaging.go             # PRIVMSG, NOTICE
│       └── channel.go              # JOIN, PART, TOPIC, NAMES, LIST
```

**Phased scope:** This plan implements the core loop (connect, register, join channels, chat) plus MOTD and LUSERS (sent in the welcome burst). MODE, WHO, WHOIS, KICK, OPER, KILL are deferred to a follow-up plan — the spec is updated to reflect this phasing.

---

### Task 1: Project Scaffold & Go Module

**Files:**
- Create: `ircd/go.mod`
- Create: `ircd/main.go`
- Create: `ircd/Taskfile.yml`
- Create: `ircd/config.yml.example`
- Modify: `go.work` (add `./ircd`)

- [ ] **Step 1: Create go.mod**

```
module github.com/jonesrussell/north-cloud/ircd

go 1.26.1

require (
	github.com/jonesrussell/north-cloud/infrastructure v0.0.0
)

replace github.com/jonesrussell/north-cloud/infrastructure => ../infrastructure
```

- [ ] **Step 2: Create minimal main.go**

```go
package main

import (
	"fmt"
	"os"
)

var version = "dev"

func main() {
	fmt.Printf("NorthCloud IRCd %s\n", version)
	os.Exit(0)
}
```

- [ ] **Step 3: Create Taskfile.yml**

Follow north-cloud conventions: build, test, lint, dev tasks. Binary name `ircd`, output to `./bin/ircd`.

```yaml
version: '3'

vars:
  BINARY_NAME: ircd
  BUILD_DIR: ./bin

tasks:
  build:
    desc: Build the ircd binary
    cmds:
      - go build -o {{.BUILD_DIR}}/{{.BINARY_NAME}} .

  run:
    desc: Run the ircd server
    cmds:
      - go run . serve

  test:
    desc: Run all tests
    cmds:
      - go test -v ./...

  test:coverage:
    desc: Run tests with coverage
    cmds:
      - go test -coverprofile=coverage.out ./...
      - go tool cover -html=coverage.out -o coverage.html

  test:race:
    desc: Run tests with race detector
    cmds:
      - go test -race ./...

  lint:
    desc: Run linters
    cmds:
      - go fmt ./...
      - go vet ./...

  deps:
    desc: Download and tidy dependencies
    cmds:
      - go mod download
      - go mod tidy
      - go mod verify

  fmt:
    desc: Format code
    cmds:
      - go fmt ./...
      - goimports -w .
```

- [ ] **Step 4: Create config.yml.example**

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
    password: "$2a$10$placeholder"
```

- [ ] **Step 5: Add ircd to go.work**

Add `./ircd` to the `use` block in `go.work`.

- [ ] **Step 6: Add ircd to root Taskfile.yml**

Add include entry:
```yaml
  ircd:
    taskfile: ./ircd/Taskfile.yml
    dir: ./ircd
```

- [ ] **Step 7: Verify it builds**

Run: `cd ircd && go build .`
Expected: Clean build, binary runs and prints version.

- [ ] **Step 8: Commit**

```bash
git add ircd/ go.work Taskfile.yml
git commit -m "feat(ircd): scaffold Go module with Taskfile and config"
```

---

### Task 2: IRC Message Parser

**Files:**
- Create: `ircd/internal/message/message.go`
- Create: `ircd/internal/message/message_test.go`

The IRC wire format is: `[:prefix] COMMAND [params] [:trailing]\r\n`

- [ ] **Step 1: Write the failing tests**

```go
package message_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/ircd/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_SimpleCommand(t *testing.T) {
	msg, err := message.Parse("QUIT\r\n")
	require.NoError(t, err)
	assert.Equal(t, "", msg.Prefix)
	assert.Equal(t, "QUIT", msg.Command)
	assert.Empty(t, msg.Params)
}

func TestParse_CommandWithParams(t *testing.T) {
	msg, err := message.Parse("NICK jones\r\n")
	require.NoError(t, err)
	assert.Equal(t, "NICK", msg.Command)
	assert.Equal(t, []string{"jones"}, msg.Params)
}

func TestParse_CommandWithTrailing(t *testing.T) {
	msg, err := message.Parse("PRIVMSG #chat :hello world\r\n")
	require.NoError(t, err)
	assert.Equal(t, "PRIVMSG", msg.Command)
	assert.Equal(t, []string{"#chat", "hello world"}, msg.Params)
}

func TestParse_WithPrefix(t *testing.T) {
	msg, err := message.Parse(":jones PRIVMSG #chat :hello\r\n")
	require.NoError(t, err)
	assert.Equal(t, "jones", msg.Prefix)
	assert.Equal(t, "PRIVMSG", msg.Command)
	assert.Equal(t, []string{"#chat", "hello"}, msg.Params)
}

func TestParse_EmptyLine(t *testing.T) {
	_, err := message.Parse("\r\n")
	assert.Error(t, err)
}

func TestParse_NoTrailingCRLF(t *testing.T) {
	msg, err := message.Parse("NICK jones")
	require.NoError(t, err)
	assert.Equal(t, "NICK", msg.Command)
	assert.Equal(t, []string{"jones"}, msg.Params)
}

func TestMessage_String(t *testing.T) {
	msg := &message.Message{
		Prefix:  "irc.northcloud.one",
		Command: "001",
		Params:  []string{"jones", "Welcome to NorthCloud"},
	}
	assert.Equal(t, ":irc.northcloud.one 001 jones :Welcome to NorthCloud\r\n", msg.String())
}

func TestMessage_String_NoPrefix(t *testing.T) {
	msg := &message.Message{
		Command: "NICK",
		Params:  []string{"jones"},
	}
	assert.Equal(t, "NICK jones\r\n", msg.String())
}

func TestMessage_String_NoParams(t *testing.T) {
	msg := &message.Message{
		Command: "QUIT",
	}
	assert.Equal(t, "QUIT\r\n", msg.String())
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ircd && go test ./internal/message/ -v`
Expected: FAIL — package doesn't exist yet.

- [ ] **Step 3: Implement message.go**

```go
package message

import (
	"errors"
	"strings"
)

var ErrEmptyMessage = errors.New("empty message")

// Message represents a parsed IRC protocol message.
type Message struct {
	Prefix  string
	Command string
	Params  []string
}

// Parse parses a raw IRC line into a Message.
// Handles optional \r\n termination.
func Parse(raw string) (*Message, error) {
	raw = strings.TrimRight(raw, "\r\n")
	if raw == "" {
		return nil, ErrEmptyMessage
	}

	msg := &Message{}
	s := raw

	// Parse optional prefix
	if strings.HasPrefix(s, ":") {
		idx := strings.Index(s, " ")
		if idx == -1 {
			return nil, ErrEmptyMessage
		}
		msg.Prefix = s[1:idx]
		s = s[idx+1:]
	}

	// Parse command
	if idx := strings.Index(s, " "); idx != -1 {
		msg.Command = strings.ToUpper(s[:idx])
		s = s[idx+1:]
	} else {
		msg.Command = strings.ToUpper(s)
		return msg, nil
	}

	// Parse params
	for s != "" {
		if strings.HasPrefix(s, ":") {
			msg.Params = append(msg.Params, s[1:])
			break
		}
		if idx := strings.Index(s, " "); idx != -1 {
			msg.Params = append(msg.Params, s[:idx])
			s = s[idx+1:]
		} else {
			msg.Params = append(msg.Params, s)
			break
		}
	}

	return msg, nil
}

// String formats the message back to IRC wire format.
func (m *Message) String() string {
	var b strings.Builder

	if m.Prefix != "" {
		b.WriteByte(':')
		b.WriteString(m.Prefix)
		b.WriteByte(' ')
	}

	b.WriteString(m.Command)

	for i, p := range m.Params {
		b.WriteByte(' ')
		if i == len(m.Params)-1 && (strings.Contains(p, " ") || p == "" || strings.HasPrefix(p, ":")) {
			b.WriteByte(':')
		}
		b.WriteString(p)
	}

	b.WriteString("\r\n")
	return b.String()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd ircd && go test ./internal/message/ -v`
Expected: All 9 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add ircd/internal/message/
git commit -m "feat(ircd): add IRC message parser with tests"
```

---

### Task 3: Config Package

**Files:**
- Create: `ircd/internal/config/config.go`
- Create: `ircd/internal/config/config_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/ircd/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	err := os.WriteFile(path, []byte(`
server:
  name: test.irc
  network: TestNet
  listen: "127.0.0.1:6667"
`), 0644)
	require.NoError(t, err)

	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, "test.irc", cfg.Server.Name)
	assert.Equal(t, "TestNet", cfg.Server.Network)
	assert.Equal(t, "127.0.0.1:6667", cfg.Server.Listen)
}

func TestLoad_Defaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	err := os.WriteFile(path, []byte("server:\n  name: test.irc\n"), 0644)
	require.NoError(t, err)

	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:6667", cfg.Server.Listen)
	assert.Equal(t, 256, cfg.Server.MaxClients)
	assert.Equal(t, 90*time.Second, cfg.Server.PingInterval)
	assert.Equal(t, 120*time.Second, cfg.Server.PongTimeout)
}

func TestLoad_MissingName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	err := os.WriteFile(path, []byte("server:\n  listen: ':6667'\n"), 0644)
	require.NoError(t, err)

	_, err = config.Load(path)
	assert.Error(t, err)
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := config.Load("/nonexistent/config.yml")
	assert.Error(t, err)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ircd && go test ./internal/config/ -v`
Expected: FAIL — package doesn't exist.

- [ ] **Step 3: Implement config.go**

```go
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server ServerConfig `yaml:"server"`
	Opers  []OperConfig `yaml:"opers"`
}

type ServerConfig struct {
	Name         string        `yaml:"name"`
	Network      string        `yaml:"network"`
	Listen       string        `yaml:"listen"`
	MaxClients   int           `yaml:"max_clients"`
	PingInterval time.Duration `yaml:"ping_interval"`
	PongTimeout  time.Duration `yaml:"pong_timeout"`
	MOTD         string        `yaml:"motd"`
}

type OperConfig struct {
	Name     string `yaml:"name"`
	Password string `yaml:"password"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	setDefaults(cfg)

	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return cfg, nil
}

func setDefaults(cfg *Config) {
	if cfg.Server.Listen == "" {
		cfg.Server.Listen = "127.0.0.1:6667"
	}
	if cfg.Server.Network == "" {
		cfg.Server.Network = "NorthCloud"
	}
	if cfg.Server.MaxClients == 0 {
		cfg.Server.MaxClients = 256
	}
	if cfg.Server.PingInterval == 0 {
		cfg.Server.PingInterval = 90 * time.Second
	}
	if cfg.Server.PongTimeout == 0 {
		cfg.Server.PongTimeout = 120 * time.Second
	}
}

func validate(cfg *Config) error {
	if cfg.Server.Name == "" {
		return fmt.Errorf("server.name is required")
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd ircd && go test ./internal/config/ -v`
Expected: All 4 tests PASS.

- [ ] **Step 5: Run `go mod tidy` to add yaml.v3 dependency**

Run: `cd ircd && go mod tidy`

- [ ] **Step 6: Commit**

```bash
git add ircd/internal/config/ ircd/go.mod ircd/go.sum
git commit -m "feat(ircd): add config loading with defaults and validation"
```

---

### Task 4: Client Package

**Files:**
- Create: `ircd/internal/client/client.go`
- Create: `ircd/internal/client/client_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package client_test

import (
	"net"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/ircd/internal/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	server, conn := net.Pipe()
	defer server.Close()
	defer conn.Close()

	c := client.New(conn)
	assert.NotEmpty(t, c.ID())
	assert.False(t, c.Registered())
	assert.Equal(t, "", c.Nick())
}

func TestClient_SetNick(t *testing.T) {
	server, conn := net.Pipe()
	defer server.Close()
	defer conn.Close()

	c := client.New(conn)
	c.SetNick("jones")
	assert.Equal(t, "jones", c.Nick())
}

func TestClient_Register(t *testing.T) {
	server, conn := net.Pipe()
	defer server.Close()
	defer conn.Close()

	c := client.New(conn)
	c.SetNick("jones")
	c.SetUser("jones", "Russell Jones")
	assert.True(t, c.Registered())
	assert.Equal(t, "jones", c.Username())
	assert.Equal(t, "Russell Jones", c.Realname())
}

func TestClient_Send(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	c := client.New(clientConn)
	go c.StartWriteLoop()

	c.SendLine(":irc.test 001 jones :Welcome\r\n")

	buf := make([]byte, 512)
	serverConn.SetReadDeadline(time.Now().Add(time.Second))
	n, err := serverConn.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, ":irc.test 001 jones :Welcome\r\n", string(buf[:n]))
}

func TestClient_Prefix(t *testing.T) {
	server, conn := net.Pipe()
	defer server.Close()
	defer conn.Close()

	c := client.New(conn)
	c.SetNick("jones")
	c.SetUser("jones", "Russell Jones")
	// Prefix format: nick!user@host
	assert.Contains(t, c.Prefix(), "jones!jones@")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ircd && go test ./internal/client/ -v`
Expected: FAIL — package doesn't exist.

- [ ] **Step 3: Implement client.go**

```go
package client

import (
	"bufio"
	"fmt"
	"net"
	"sync"
)

const sendBufferSize = 512

// Client represents a connected IRC client.
type Client struct {
	id       string
	conn     net.Conn
	send     chan string
	nick     string
	username string
	realname string
	mu       sync.RWMutex
	quit     chan struct{}
	once     sync.Once
}

// New creates a new client from a TCP connection.
func New(conn net.Conn) *Client {
	return &Client{
		id:   conn.RemoteAddr().String(),
		conn: conn,
		send: make(chan string, sendBufferSize),
		quit: make(chan struct{}),
	}
}

func (c *Client) ID() string {
	return c.id
}

func (c *Client) Nick() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.nick
}

func (c *Client) SetNick(nick string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nick = nick
}

func (c *Client) Username() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.username
}

func (c *Client) Realname() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.realname
}

func (c *Client) SetUser(username, realname string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.username = username
	c.realname = realname
}

func (c *Client) Registered() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.nick != "" && c.username != ""
}

// Prefix returns the full IRC prefix: nick!user@host
func (c *Client) Prefix() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	host := c.conn.RemoteAddr().String()
	return fmt.Sprintf("%s!%s@%s", c.nick, c.username, host)
}

// Hostname returns the remote address of the client.
func (c *Client) Hostname() string {
	return c.conn.RemoteAddr().String()
}

// SendLine queues a line to be sent to the client.
// Returns false if the send buffer is full or client is closing.
func (c *Client) SendLine(line string) bool {
	select {
	case <-c.quit:
		return false
	default:
	}
	select {
	case c.send <- line:
		return true
	case <-c.quit:
		return false
	default:
		return false
	}
}

// StartWriteLoop drains the send channel and writes to the connection.
// Blocks until the send channel is closed or quit is signaled.
func (c *Client) StartWriteLoop() {
	for {
		select {
		case line, ok := <-c.send:
			if !ok {
				return
			}
			_, _ = c.conn.Write([]byte(line))
		case <-c.quit:
			return
		}
	}
}

// ReadLines returns a scanner that yields lines from the connection.
func (c *Client) ReadLines() *bufio.Scanner {
	return bufio.NewScanner(c.conn)
}

// Close shuts down the client connection. Safe to call multiple times.
func (c *Client) Close() {
	c.once.Do(func() {
		close(c.quit)
		c.conn.Close()
	})
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd ircd && go test ./internal/client/ -v`
Expected: All 5 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add ircd/internal/client/
git commit -m "feat(ircd): add client package with read/write loops"
```

---

### Task 5: Channel Package

**Files:**
- Create: `ircd/internal/channel/channel.go`
- Create: `ircd/internal/channel/channel_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package channel_test

import (
	"net"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/ircd/internal/channel"
	"github.com/jonesrussell/north-cloud/ircd/internal/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T, nick string) (*client.Client, net.Conn) {
	t.Helper()
	serverConn, clientConn := net.Pipe()
	t.Cleanup(func() {
		serverConn.Close()
		clientConn.Close()
	})
	c := client.New(clientConn)
	c.SetNick(nick)
	c.SetUser(nick, nick)
	go c.StartWriteLoop()
	return c, serverConn
}

func TestNewChannel(t *testing.T) {
	ch := channel.New("#test")
	assert.Equal(t, "#test", ch.Name())
	assert.Equal(t, 0, ch.MemberCount())
}

func TestChannel_JoinAndPart(t *testing.T) {
	ch := channel.New("#test")
	c, _ := newTestClient(t, "jones")

	ch.Join(c)
	assert.Equal(t, 1, ch.MemberCount())
	assert.True(t, ch.HasMember(c))

	ch.Part(c)
	assert.Equal(t, 0, ch.MemberCount())
	assert.False(t, ch.HasMember(c))
}

func TestChannel_Broadcast(t *testing.T) {
	ch := channel.New("#test")
	c1, s1 := newTestClient(t, "alice")
	c2, s2 := newTestClient(t, "bob")

	ch.Join(c1)
	ch.Join(c2)

	// Broadcast from alice — bob should receive, alice should not
	ch.Broadcast(c1, ":alice PRIVMSG #test :hello\r\n")

	buf := make([]byte, 512)
	s2.SetReadDeadline(time.Now().Add(time.Second))
	n, err := s2.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, ":alice PRIVMSG #test :hello\r\n", string(buf[:n]))

	// alice should NOT receive her own message
	s1.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	_, err = s1.Read(buf)
	assert.Error(t, err) // timeout = no data
}

func TestChannel_Topic(t *testing.T) {
	ch := channel.New("#test")
	assert.Equal(t, "", ch.Topic())

	ch.SetTopic("Welcome!")
	assert.Equal(t, "Welcome!", ch.Topic())
}

func TestChannel_Members(t *testing.T) {
	ch := channel.New("#test")
	c1, _ := newTestClient(t, "alice")
	c2, _ := newTestClient(t, "bob")

	ch.Join(c1)
	ch.Join(c2)

	members := ch.Members()
	assert.Len(t, members, 2)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ircd && go test ./internal/channel/ -v`
Expected: FAIL — package doesn't exist.

- [ ] **Step 3: Implement channel.go**

```go
package channel

import (
	"sync"

	"github.com/jonesrussell/north-cloud/ircd/internal/client"
)

// Channel represents an IRC channel.
type Channel struct {
	name    string
	topic   string
	members map[*client.Client]bool
	mu      sync.RWMutex
}

// New creates a new channel.
func New(name string) *Channel {
	return &Channel{
		name:    name,
		members: make(map[*client.Client]bool),
	}
}

func (ch *Channel) Name() string {
	return ch.name
}

func (ch *Channel) Topic() string {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	return ch.topic
}

func (ch *Channel) SetTopic(topic string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.topic = topic
}

func (ch *Channel) MemberCount() int {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	return len(ch.members)
}

func (ch *Channel) HasMember(c *client.Client) bool {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	return ch.members[c]
}

// Join adds a client to the channel.
func (ch *Channel) Join(c *client.Client) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.members[c] = true
}

// Part removes a client from the channel.
func (ch *Channel) Part(c *client.Client) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	delete(ch.members, c)
}

// Members returns a snapshot of all members.
func (ch *Channel) Members() []*client.Client {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	members := make([]*client.Client, 0, len(ch.members))
	for c := range ch.members {
		members = append(members, c)
	}
	return members
}

// Broadcast sends a message to all members except the sender.
func (ch *Channel) Broadcast(sender *client.Client, line string) {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	for c := range ch.members {
		if c != sender {
			c.SendLine(line)
		}
	}
}

// BroadcastAll sends a message to all members including the sender.
func (ch *Channel) BroadcastAll(line string) {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	for c := range ch.members {
		c.SendLine(line)
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd ircd && go test ./internal/channel/ -v`
Expected: All 5 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add ircd/internal/channel/
git commit -m "feat(ircd): add channel package with join/part/broadcast"
```

---

### Task 6: Command Handler Framework

**Files:**
- Create: `ircd/internal/command/handler.go`
- Create: `ircd/internal/command/handler_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package command_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/ircd/internal/command"
	"github.com/jonesrussell/north-cloud/ircd/internal/message"
	"github.com/stretchr/testify/assert"
)

func TestRegistry_RegisterAndLookup(t *testing.T) {
	reg := command.NewRegistry()
	called := false
	reg.Register("TEST", func(ctx *command.Context) {
		called = true
	})

	handler, ok := reg.Lookup("TEST")
	assert.True(t, ok)

	handler(&command.Context{
		Message: &message.Message{Command: "TEST"},
	})
	assert.True(t, called)
}

func TestRegistry_LookupCaseInsensitive(t *testing.T) {
	reg := command.NewRegistry()
	reg.Register("NICK", func(ctx *command.Context) {})

	_, ok := reg.Lookup("nick")
	assert.True(t, ok)

	_, ok = reg.Lookup("Nick")
	assert.True(t, ok)
}

func TestRegistry_LookupUnknown(t *testing.T) {
	reg := command.NewRegistry()
	_, ok := reg.Lookup("UNKNOWN")
	assert.False(t, ok)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ircd && go test ./internal/command/ -v`
Expected: FAIL — package doesn't exist.

- [ ] **Step 3: Implement handler.go**

```go
package command

import (
	"strings"

	"github.com/jonesrussell/north-cloud/ircd/internal/client"
	"github.com/jonesrussell/north-cloud/ircd/internal/message"
)

// ServerInterface defines what command handlers need from the server.
// Avoids circular imports between command and server packages.
type ServerInterface interface {
	ServerName() string
	NetworkName() string
	MOTD() string
	ClientCount() int

	// Client operations
	FindClientByNick(nick string) *client.Client
	UnregisterClient(c *client.Client)
	ChangeNick(c *client.Client, newNick string) error

	// Channel operations
	JoinChannel(c *client.Client, name string)
	PartChannel(c *client.Client, name string, reason string)
	ChannelNames(name string) []string
	ChannelTopic(name string) string
	SetChannelTopic(c *client.Client, name string, topic string)
	ListChannels() []ChannelInfo
	BroadcastToChannel(sender *client.Client, channelName string, line string) bool
}

// ChannelInfo holds summary info for LIST replies.
type ChannelInfo struct {
	Name       string
	MemberCount int
	Topic      string
}

// Context is passed to every command handler.
type Context struct {
	Server  ServerInterface
	Client  *client.Client
	Message *message.Message
}

// HandlerFunc is the signature for command handlers.
type HandlerFunc func(ctx *Context)

// Registry maps IRC commands to handler functions.
type Registry struct {
	handlers map[string]HandlerFunc
}

// NewRegistry creates an empty command registry.
func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[string]HandlerFunc),
	}
}

// Register adds a handler for a command (stored uppercase).
func (r *Registry) Register(command string, handler HandlerFunc) {
	r.handlers[strings.ToUpper(command)] = handler
}

// Lookup finds the handler for a command (case-insensitive).
func (r *Registry) Lookup(command string) (HandlerFunc, bool) {
	h, ok := r.handlers[strings.ToUpper(command)]
	return h, ok
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd ircd && go test ./internal/command/ -v`
Expected: All 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add ircd/internal/command/
git commit -m "feat(ircd): add command handler registry with context"
```

---

### Task 7: Registration Commands (NICK, USER, QUIT, PING/PONG)

**Files:**
- Create: `ircd/internal/command/registration.go`
- Create: `ircd/internal/command/registration_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package command_test

import (
	"net"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/ircd/internal/client"
	"github.com/jonesrussell/north-cloud/ircd/internal/command"
	"github.com/jonesrussell/north-cloud/ircd/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockServer implements command.ServerInterface for testing.
type mockServer struct {
	name      string
	network   string
	motd      string
	clients   map[string]*client.Client
	nickErr   error
}

func newMockServer() *mockServer {
	return &mockServer{
		name:    "test.irc",
		network: "TestNet",
		motd:    "Welcome!",
		clients: make(map[string]*client.Client),
	}
}

func (s *mockServer) ServerName() string                { return s.name }
func (s *mockServer) NetworkName() string               { return s.network }
func (s *mockServer) MOTD() string                      { return s.motd }
func (s *mockServer) ClientCount() int                  { return len(s.clients) }
func (s *mockServer) FindClientByNick(nick string) *client.Client { return s.clients[nick] }
func (s *mockServer) UnregisterClient(c *client.Client)           {}
func (s *mockServer) ChangeNick(c *client.Client, nick string) error {
	if s.nickErr != nil {
		return s.nickErr
	}
	delete(s.clients, c.Nick())
	c.SetNick(nick)
	s.clients[nick] = c
	return nil
}
func (s *mockServer) JoinChannel(c *client.Client, name string)                          {}
func (s *mockServer) PartChannel(c *client.Client, name string, reason string)           {}
func (s *mockServer) ChannelNames(name string) []string                                  { return nil }
func (s *mockServer) ChannelTopic(name string) string                                    { return "" }
func (s *mockServer) SetChannelTopic(c *client.Client, name string, topic string)        {}
func (s *mockServer) ListChannels() []command.ChannelInfo                                { return nil }
func (s *mockServer) BroadcastToChannel(sender *client.Client, ch string, line string) bool { return true }

func newTestCtx(t *testing.T, srv *mockServer) (*command.Context, net.Conn) {
	t.Helper()
	serverConn, clientConn := net.Pipe()
	t.Cleanup(func() {
		serverConn.Close()
		clientConn.Close()
	})
	c := client.New(clientConn)
	go c.StartWriteLoop()
	return &command.Context{
		Server:  srv,
		Client:  c,
	}, serverConn
}

func readLine(t *testing.T, conn net.Conn) string {
	t.Helper()
	buf := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(time.Second))
	n, err := conn.Read(buf)
	require.NoError(t, err)
	return string(buf[:n])
}

func TestHandleNick_SetsNick(t *testing.T) {
	srv := newMockServer()
	ctx, _ := newTestCtx(t, srv)
	ctx.Message = &message.Message{Command: "NICK", Params: []string{"jones"}}

	command.HandleNick(ctx)
	assert.Equal(t, "jones", ctx.Client.Nick())
}

func TestHandleNick_NoParams(t *testing.T) {
	srv := newMockServer()
	ctx, conn := newTestCtx(t, srv)
	ctx.Message = &message.Message{Command: "NICK"}

	command.HandleNick(ctx)

	line := readLine(t, conn)
	assert.Contains(t, line, "431") // ERR_NONICKNAMEGIVEN
}

func TestHandlePing(t *testing.T) {
	srv := newMockServer()
	ctx, conn := newTestCtx(t, srv)
	ctx.Message = &message.Message{Command: "PING", Params: []string{"test.irc"}}

	command.HandlePing(ctx)

	line := readLine(t, conn)
	assert.Contains(t, line, "PONG")
	assert.Contains(t, line, "test.irc")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ircd && go test ./internal/command/ -v -run "TestHandle"`
Expected: FAIL — functions don't exist.

- [ ] **Step 3: Implement registration.go**

```go
package command

import (
	"fmt"
	"strings"
)

// HandleNick handles the NICK command.
func HandleNick(ctx *Context) {
	if len(ctx.Message.Params) == 0 {
		ctx.reply("431", ctx.clientNick(), "No nickname given")
		return
	}

	nick := ctx.Message.Params[0]

	if !isValidNick(nick) {
		ctx.reply("432", ctx.clientNick(), nick, "Erroneous nickname")
		return
	}

	if existing := ctx.Server.FindClientByNick(nick); existing != nil && existing != ctx.Client {
		ctx.reply("433", ctx.clientNick(), nick, "Nickname is already in use")
		return
	}

	wasRegistered := ctx.Client.Registered()
	if err := ctx.Server.ChangeNick(ctx.Client, nick); err != nil {
		ctx.reply("433", ctx.clientNick(), nick, "Nickname is already in use")
		return
	}

	if !wasRegistered && ctx.Client.Registered() {
		sendWelcome(ctx)
	}
}

// HandleUser handles the USER command.
func HandleUser(ctx *Context) {
	if ctx.Client.Registered() {
		ctx.reply("462", ctx.Client.Nick(), "You may not reregister")
		return
	}

	if len(ctx.Message.Params) < 4 {
		ctx.reply("461", ctx.clientNick(), "USER", "Not enough parameters")
		return
	}

	username := ctx.Message.Params[0]
	realname := ctx.Message.Params[3]

	ctx.Client.SetUser(username, realname)

	if ctx.Client.Registered() {
		sendWelcome(ctx)
	}
}

// HandleQuit handles the QUIT command.
func HandleQuit(ctx *Context) {
	reason := "Client quit"
	if len(ctx.Message.Params) > 0 {
		reason = ctx.Message.Params[0]
	}
	ctx.Client.SendLine(fmt.Sprintf("ERROR :Closing link: %s (%s)\r\n", ctx.Client.Hostname(), reason))
	ctx.Server.UnregisterClient(ctx.Client)
}

// HandlePing handles the PING command.
func HandlePing(ctx *Context) {
	token := ctx.Server.ServerName()
	if len(ctx.Message.Params) > 0 {
		token = ctx.Message.Params[0]
	}
	ctx.Client.SendLine(fmt.Sprintf(":%s PONG %s :%s\r\n", ctx.Server.ServerName(), ctx.Server.ServerName(), token))
}

// HandlePass handles the PASS command. Stored but not used for connection auth in MVP.
func HandlePass(ctx *Context) {
	if ctx.Client.Registered() {
		ctx.reply("462", ctx.Client.Nick(), "You may not reregister")
		return
	}
	// PASS is accepted but not validated in MVP (open server).
}

// HandlePong handles the PONG command (updates last activity, no response needed).
func HandlePong(ctx *Context) {
	// PONG received — keepalive logic is handled by the server's ping timer.
}

func sendWelcome(ctx *Context) {
	nick := ctx.Client.Nick()
	sn := ctx.Server.ServerName()
	net := ctx.Server.NetworkName()

	ctx.reply("001", nick, fmt.Sprintf("Welcome to the %s IRC Network %s", net, ctx.Client.Prefix()))
	ctx.reply("002", nick, fmt.Sprintf("Your host is %s, running NorthCloud IRCd v0.1.0", sn))
	ctx.reply("003", nick, "This server was created recently")
	ctx.reply("004", nick, sn, "NorthCloudIRCd-0.1.0", "io", "otn")
	ctx.reply("005", nick, "CHANTYPES=#", "CHANMODES=,,,nt", "PREFIX=(o)@", fmt.Sprintf("NETWORK=%s", net), "are supported by this server")

	// Send MOTD
	motd := ctx.Server.MOTD()
	if motd != "" {
		ctx.reply("375", nick, fmt.Sprintf("- %s Message of the day -", sn))
		for _, line := range strings.Split(motd, "\n") {
			if line != "" {
				ctx.reply("372", nick, fmt.Sprintf("- %s", line))
			}
		}
		ctx.reply("376", nick, "End of /MOTD command")
	}

	ctx.reply("251", nick, fmt.Sprintf("There are %d users on 1 server", ctx.Server.ClientCount()))
}

// reply sends a numeric reply to the client.
func (ctx *Context) reply(numeric string, params ...string) {
	msg := fmt.Sprintf(":%s %s", ctx.Server.ServerName(), numeric)
	for i, p := range params {
		if i == len(params)-1 && strings.Contains(p, " ") {
			msg += " :" + p
		} else {
			msg += " " + p
		}
	}
	msg += "\r\n"
	ctx.Client.SendLine(msg)
}

func (ctx *Context) clientNick() string {
	nick := ctx.Client.Nick()
	if nick == "" {
		return "*"
	}
	return nick
}

func isValidNick(nick string) bool {
	if nick == "" || len(nick) > 30 {
		return false
	}
	if nick[0] >= '0' && nick[0] <= '9' {
		return false
	}
	for _, r := range nick {
		if r == ' ' || r == ',' || r == '*' || r == '?' || r == '!' || r == '@' || r == '#' || r == '&' || r == ':' {
			return false
		}
	}
	return true
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd ircd && go test ./internal/command/ -v`
Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add ircd/internal/command/
git commit -m "feat(ircd): add NICK, USER, QUIT, PING/PONG handlers"
```

---

### Task 8: Messaging Commands (PRIVMSG, NOTICE)

**Files:**
- Create: `ircd/internal/command/messaging.go`
- Create: `ircd/internal/command/messaging_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package command_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/ircd/internal/command"
	"github.com/jonesrussell/north-cloud/ircd/internal/message"
	"github.com/stretchr/testify/assert"
)

func TestHandlePrivmsg_ToUser(t *testing.T) {
	srv := newMockServer()
	ctx, _ := newTestCtx(t, srv)
	ctx.Client.SetNick("alice")
	ctx.Client.SetUser("alice", "Alice")

	// Create target user
	targetCtx, targetConn := newTestCtx(t, srv)
	targetCtx.Client.SetNick("bob")
	targetCtx.Client.SetUser("bob", "Bob")
	srv.clients["bob"] = targetCtx.Client

	ctx.Message = &message.Message{Command: "PRIVMSG", Params: []string{"bob", "hello bob"}}
	command.HandlePrivmsg(ctx)

	line := readLine(t, targetConn)
	assert.Contains(t, line, "PRIVMSG bob :hello bob")
	assert.Contains(t, line, "alice")
}

func TestHandlePrivmsg_NoRecipient(t *testing.T) {
	srv := newMockServer()
	ctx, conn := newTestCtx(t, srv)
	ctx.Client.SetNick("alice")
	ctx.Client.SetUser("alice", "Alice")
	ctx.Message = &message.Message{Command: "PRIVMSG"}

	command.HandlePrivmsg(ctx)

	line := readLine(t, conn)
	assert.Contains(t, line, "411") // ERR_NORECIPIENT
}

func TestHandlePrivmsg_NoSuchNick(t *testing.T) {
	srv := newMockServer()
	ctx, conn := newTestCtx(t, srv)
	ctx.Client.SetNick("alice")
	ctx.Client.SetUser("alice", "Alice")
	ctx.Message = &message.Message{Command: "PRIVMSG", Params: []string{"nobody", "hello"}}

	command.HandlePrivmsg(ctx)

	line := readLine(t, conn)
	assert.Contains(t, line, "401") // ERR_NOSUCHNICK
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ircd && go test ./internal/command/ -v -run "TestHandlePrivmsg"`
Expected: FAIL — function doesn't exist.

- [ ] **Step 3: Implement messaging.go**

```go
package command

import (
	"fmt"
	"strings"
)

// HandlePrivmsg handles the PRIVMSG command.
func HandlePrivmsg(ctx *Context) {
	handleMessage(ctx, "PRIVMSG")
}

// HandleNotice handles the NOTICE command.
// Per RFC, NOTICE must never generate automatic replies.
func HandleNotice(ctx *Context) {
	handleMessage(ctx, "NOTICE")
}

func handleMessage(ctx *Context, cmd string) {
	if len(ctx.Message.Params) == 0 {
		ctx.reply("411", ctx.Client.Nick(), fmt.Sprintf("No recipient given (%s)", cmd))
		return
	}
	if len(ctx.Message.Params) < 2 {
		ctx.reply("412", ctx.Client.Nick(), "No text to send")
		return
	}

	target := ctx.Message.Params[0]
	text := ctx.Message.Params[1]
	line := fmt.Sprintf(":%s %s %s :%s\r\n", ctx.Client.Prefix(), cmd, target, text)

	if strings.HasPrefix(target, "#") || strings.HasPrefix(target, "&") {
		// Channel message
		if !ctx.Server.BroadcastToChannel(ctx.Client, target, line) {
			ctx.reply("403", ctx.Client.Nick(), target, "No such channel")
		}
	} else {
		// Private message
		recipient := ctx.Server.FindClientByNick(target)
		if recipient == nil {
			ctx.reply("401", ctx.Client.Nick(), target, "No such nick/channel")
			return
		}
		recipient.SendLine(line)
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd ircd && go test ./internal/command/ -v -run "TestHandlePrivmsg"`
Expected: All 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add ircd/internal/command/messaging.go ircd/internal/command/messaging_test.go
git commit -m "feat(ircd): add PRIVMSG and NOTICE handlers"
```

---

### Task 9: Channel Commands (JOIN, PART, TOPIC, NAMES, LIST)

**Files:**
- Create: `ircd/internal/command/channel.go`
- Create: `ircd/internal/command/channel_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package command_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/ircd/internal/command"
	"github.com/jonesrussell/north-cloud/ircd/internal/message"
	"github.com/stretchr/testify/assert"
)

func TestHandleJoin_NoParams(t *testing.T) {
	srv := newMockServer()
	ctx, conn := newTestCtx(t, srv)
	ctx.Client.SetNick("jones")
	ctx.Client.SetUser("jones", "Jones")
	ctx.Message = &message.Message{Command: "JOIN"}

	command.HandleJoin(ctx)

	line := readLine(t, conn)
	assert.Contains(t, line, "461") // ERR_NEEDMOREPARAMS
}

func TestHandleJoin_InvalidChannel(t *testing.T) {
	srv := newMockServer()
	ctx, conn := newTestCtx(t, srv)
	ctx.Client.SetNick("jones")
	ctx.Client.SetUser("jones", "Jones")
	ctx.Message = &message.Message{Command: "JOIN", Params: []string{"nochanprefix"}}

	command.HandleJoin(ctx)

	line := readLine(t, conn)
	assert.Contains(t, line, "403") // ERR_NOSUCHCHANNEL
}

func TestHandlePart_NoParams(t *testing.T) {
	srv := newMockServer()
	ctx, conn := newTestCtx(t, srv)
	ctx.Client.SetNick("jones")
	ctx.Client.SetUser("jones", "Jones")
	ctx.Message = &message.Message{Command: "PART"}

	command.HandlePart(ctx)

	line := readLine(t, conn)
	assert.Contains(t, line, "461") // ERR_NEEDMOREPARAMS
}

func TestHandleList(t *testing.T) {
	srv := newMockServer()
	srv.channelList = []command.ChannelInfo{
		{Name: "#test", MemberCount: 3, Topic: "Testing"},
	}
	ctx, conn := newTestCtx(t, srv)
	ctx.Client.SetNick("jones")
	ctx.Client.SetUser("jones", "Jones")
	ctx.Message = &message.Message{Command: "LIST"}

	command.HandleList(ctx)

	// Should get 321 (list start), 322 (channel entry), 323 (list end)
	line := readLine(t, conn)
	assert.Contains(t, line, "321")
}
```

Update the mockServer to add `channelList`:

```go
// Add to mockServer struct:
type mockServer struct {
	// ... existing fields
	channelList []command.ChannelInfo
	joinedChans map[string][]string
}

func (s *mockServer) ListChannels() []command.ChannelInfo { return s.channelList }
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ircd && go test ./internal/command/ -v -run "TestHandleJoin|TestHandlePart|TestHandleList"`
Expected: FAIL — functions don't exist.

- [ ] **Step 3: Implement channel.go**

```go
package command

import (
	"fmt"
	"strings"
)

// HandleJoin handles the JOIN command.
func HandleJoin(ctx *Context) {
	if len(ctx.Message.Params) == 0 {
		ctx.reply("461", ctx.Client.Nick(), "JOIN", "Not enough parameters")
		return
	}

	channels := strings.Split(ctx.Message.Params[0], ",")
	for _, ch := range channels {
		ch = strings.TrimSpace(ch)
		if !strings.HasPrefix(ch, "#") && !strings.HasPrefix(ch, "&") {
			ctx.reply("403", ctx.Client.Nick(), ch, "No such channel")
			continue
		}

		ctx.Server.JoinChannel(ctx.Client, ch)
	}
}

// HandlePart handles the PART command.
func HandlePart(ctx *Context) {
	if len(ctx.Message.Params) == 0 {
		ctx.reply("461", ctx.Client.Nick(), "PART", "Not enough parameters")
		return
	}

	reason := ""
	if len(ctx.Message.Params) > 1 {
		reason = ctx.Message.Params[1]
	}

	channels := strings.Split(ctx.Message.Params[0], ",")
	for _, ch := range channels {
		ch = strings.TrimSpace(ch)
		ctx.Server.PartChannel(ctx.Client, ch, reason)
	}
}

// HandleTopic handles the TOPIC command.
func HandleTopic(ctx *Context) {
	if len(ctx.Message.Params) == 0 {
		ctx.reply("461", ctx.Client.Nick(), "TOPIC", "Not enough parameters")
		return
	}

	channelName := ctx.Message.Params[0]

	if len(ctx.Message.Params) == 1 {
		// Query topic
		topic := ctx.Server.ChannelTopic(channelName)
		if topic == "" {
			ctx.reply("331", ctx.Client.Nick(), channelName, "No topic is set")
		} else {
			ctx.reply("332", ctx.Client.Nick(), channelName, topic)
		}
		return
	}

	// Set topic
	ctx.Server.SetChannelTopic(ctx.Client, channelName, ctx.Message.Params[1])
}

// HandleNames handles the NAMES command.
func HandleNames(ctx *Context) {
	if len(ctx.Message.Params) == 0 {
		ctx.reply("461", ctx.Client.Nick(), "NAMES", "Not enough parameters")
		return
	}

	channelName := ctx.Message.Params[0]
	names := ctx.Server.ChannelNames(channelName)

	if names != nil {
		ctx.reply("353", ctx.Client.Nick(), "=", channelName, strings.Join(names, " "))
	}
	ctx.reply("366", ctx.Client.Nick(), channelName, "End of /NAMES list")
}

// HandleList handles the LIST command.
func HandleList(ctx *Context) {
	ctx.reply("321", ctx.Client.Nick(), "Channel", "Users  Name")

	for _, ch := range ctx.Server.ListChannels() {
		ctx.Client.SendLine(fmt.Sprintf(":%s 322 %s %s %d :%s\r\n",
			ctx.Server.ServerName(), ctx.Client.Nick(),
			ch.Name, ch.MemberCount, ch.Topic))
	}

	ctx.reply("323", ctx.Client.Nick(), "End of /LIST")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd ircd && go test ./internal/command/ -v`
Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add ircd/internal/command/channel.go ircd/internal/command/channel_test.go
git commit -m "feat(ircd): add JOIN, PART, TOPIC, NAMES, LIST handlers"
```

---

### Task 10: Server Package (Wiring Everything Together)

**Files:**
- Create: `ircd/internal/server/server.go`
- Create: `ircd/internal/server/server_test.go`
- Modify: `ircd/main.go` (wire up and start server)

The server implements `command.ServerInterface`, owns the client/channel maps, and runs the TCP accept loop.

- [ ] **Step 1: Write the integration test**

```go
package server_test

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/ircd/internal/config"
	"github.com/jonesrussell/north-cloud/ircd/internal/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func startTestServer(t *testing.T) (string, func()) {
	t.Helper()
	cfg := &config.Config{
		Server: config.ServerConfig{
			Name:         "test.irc",
			Network:      "TestNet",
			Listen:       "127.0.0.1:0", // random port
			MaxClients:   10,
			PingInterval: 5 * time.Minute, // disable for tests
			PongTimeout:  5 * time.Minute,
			MOTD:         "Welcome!",
		},
	}

	srv := server.New(cfg, nil) // nil logger = no logging
	addr, err := srv.Start()
	require.NoError(t, err)

	return addr, func() { srv.Shutdown() }
}

func connectAndRegister(t *testing.T, addr, nick string) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	fmt.Fprintf(conn, "NICK %s\r\n", nick)
	fmt.Fprintf(conn, "USER %s 0 * :Test User\r\n", nick)

	// Drain welcome burst
	scanner := bufio.NewScanner(conn)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "376") || strings.Contains(line, "422") {
			break // End of MOTD or no MOTD
		}
	}
	conn.SetReadDeadline(time.Time{}) // clear deadline
	return conn
}

func TestServer_ConnectAndRegister(t *testing.T) {
	addr, stop := startTestServer(t)
	defer stop()

	conn := connectAndRegister(t, addr, "jones")
	assert.NotNil(t, conn)
}

func TestServer_PrivateMessage(t *testing.T) {
	addr, stop := startTestServer(t)
	defer stop()

	alice := connectAndRegister(t, addr, "alice")
	bob := connectAndRegister(t, addr, "bob")

	fmt.Fprintf(alice, "PRIVMSG bob :hello bob\r\n")

	bob.SetReadDeadline(time.Now().Add(2 * time.Second))
	scanner := bufio.NewScanner(bob)
	require.True(t, scanner.Scan())
	assert.Contains(t, scanner.Text(), "PRIVMSG bob :hello bob")
	assert.Contains(t, scanner.Text(), "alice")
}

func TestServer_ChannelChat(t *testing.T) {
	addr, stop := startTestServer(t)
	defer stop()

	alice := connectAndRegister(t, addr, "alice")
	bob := connectAndRegister(t, addr, "bob")

	fmt.Fprintf(alice, "JOIN #test\r\n")
	// Drain JOIN responses for alice
	alice.SetReadDeadline(time.Now().Add(time.Second))
	ascanner := bufio.NewScanner(alice)
	for ascanner.Scan() {
		if strings.Contains(ascanner.Text(), "366") {
			break
		}
	}
	alice.SetReadDeadline(time.Time{})

	fmt.Fprintf(bob, "JOIN #test\r\n")
	// Drain JOIN responses for bob + alice's JOIN notification
	bob.SetReadDeadline(time.Now().Add(time.Second))
	bscanner := bufio.NewScanner(bob)
	for bscanner.Scan() {
		if strings.Contains(bscanner.Text(), "366") {
			break
		}
	}
	bob.SetReadDeadline(time.Time{})

	// Alice sends message to channel
	fmt.Fprintf(alice, "PRIVMSG #test :hello channel\r\n")

	// Bob should receive it
	bob.SetReadDeadline(time.Now().Add(2 * time.Second))
	require.True(t, bscanner.Scan())
	assert.Contains(t, bscanner.Text(), "PRIVMSG #test :hello channel")
}

func TestServer_DuplicateNick(t *testing.T) {
	addr, stop := startTestServer(t)
	defer stop()

	connectAndRegister(t, addr, "jones")

	// Second connection tries same nick
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	require.NoError(t, err)
	defer conn.Close()

	fmt.Fprintf(conn, "NICK jones\r\n")

	conn.SetReadDeadline(time.Now().Add(time.Second))
	scanner := bufio.NewScanner(conn)
	require.True(t, scanner.Scan())
	assert.Contains(t, scanner.Text(), "433") // ERR_NICKNAMEINUSE
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ircd && go test ./internal/server/ -v`
Expected: FAIL — package doesn't exist.

- [ ] **Step 3: Implement server.go**

This is the largest file. It implements `command.ServerInterface`, manages the TCP listener, client registry, channel registry, and dispatches commands.

```go
package server

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/ircd/internal/channel"
	"github.com/jonesrussell/north-cloud/ircd/internal/client"
	"github.com/jonesrussell/north-cloud/ircd/internal/command"
	"github.com/jonesrussell/north-cloud/ircd/internal/config"
	"github.com/jonesrussell/north-cloud/ircd/internal/message"
)

type Server struct {
	cfg      *config.Config
	log      infralogger.Logger
	listener net.Listener
	registry *command.Registry

	clients    map[string]*client.Client // nick -> client
	clientsMu  sync.RWMutex
	channels   map[string]*channel.Channel // name -> channel
	channelsMu sync.RWMutex

	quit chan struct{}
	wg   sync.WaitGroup // tracks active connection goroutines
}

func New(cfg *config.Config, log infralogger.Logger) *Server {
	s := &Server{
		cfg:      cfg,
		log:      log,
		clients:  make(map[string]*client.Client),
		channels: make(map[string]*channel.Channel),
		quit:     make(chan struct{}),
	}

	s.registry = command.NewRegistry()
	s.registry.Register("PASS", command.HandlePass)
	s.registry.Register("NICK", command.HandleNick)
	s.registry.Register("USER", command.HandleUser)
	s.registry.Register("QUIT", command.HandleQuit)
	s.registry.Register("PING", command.HandlePing)
	s.registry.Register("PONG", command.HandlePong)
	s.registry.Register("PRIVMSG", command.HandlePrivmsg)
	s.registry.Register("NOTICE", command.HandleNotice)
	s.registry.Register("JOIN", command.HandleJoin)
	s.registry.Register("PART", command.HandlePart)
	s.registry.Register("TOPIC", command.HandleTopic)
	s.registry.Register("NAMES", command.HandleNames)
	s.registry.Register("LIST", command.HandleList)

	return s
}

// Start begins listening and accepting connections. Returns the address.
func (s *Server) Start() (string, error) {
	ln, err := net.Listen("tcp", s.cfg.Server.Listen)
	if err != nil {
		return "", fmt.Errorf("listen: %w", err)
	}
	s.listener = ln

	go s.acceptLoop()

	return ln.Addr().String(), nil
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown() {
	close(s.quit)
	s.listener.Close()

	s.clientsMu.RLock()
	for _, c := range s.clients {
		c.SendLine("ERROR :Server shutting down\r\n")
		c.Close()
	}
	s.clientsMu.RUnlock()

	s.wg.Wait() // wait for all connection goroutines to finish
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.quit:
				return
			default:
				s.logError("accept error", err)
				continue
			}
		}
		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	c := client.New(conn)
	go c.StartWriteLoop()

	scanner := c.ReadLines()
	for scanner.Scan() {
		raw := scanner.Text()
		if raw == "" {
			continue
		}

		// Truncate to 512 bytes
		if len(raw) > 510 {
			raw = raw[:510]
		}

		msg, err := message.Parse(raw)
		if err != nil {
			continue
		}

		handler, ok := s.registry.Lookup(msg.Command)
		if !ok {
			if c.Registered() {
				c.SendLine(fmt.Sprintf(":%s 421 %s %s :Unknown command\r\n",
					s.cfg.Server.Name, c.Nick(), msg.Command))
			}
			continue
		}

		ctx := &command.Context{
			Server:  s,
			Client:  c,
			Message: msg,
		}
		handler(ctx)
	}

	// Client disconnected
	s.UnregisterClient(c)
}

// --- ServerInterface implementation ---

func (s *Server) ServerName() string  { return s.cfg.Server.Name }
func (s *Server) NetworkName() string { return s.cfg.Server.Network }
func (s *Server) MOTD() string        { return s.cfg.Server.MOTD }

func (s *Server) ClientCount() int {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()
	return len(s.clients)
}

func (s *Server) FindClientByNick(nick string) *client.Client {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()
	return s.clients[strings.ToLower(nick)]
}

func (s *Server) ChangeNick(c *client.Client, newNick string) error {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	lower := strings.ToLower(newNick)
	if existing, exists := s.clients[lower]; exists && existing != c {
		return errors.New("nick in use")
	}

	// Remove old nick
	oldNick := strings.ToLower(c.Nick())
	if oldNick != "" {
		delete(s.clients, oldNick)
	}

	c.SetNick(newNick)
	s.clients[lower] = c
	return nil
}

func (s *Server) UnregisterClient(c *client.Client) {
	s.clientsMu.Lock()
	nick := strings.ToLower(c.Nick())
	delete(s.clients, nick)
	s.clientsMu.Unlock()

	// Remove from all channels and notify
	s.channelsMu.RLock()
	for _, ch := range s.channels {
		if ch.HasMember(c) {
			ch.BroadcastAll(fmt.Sprintf(":%s QUIT :Client quit\r\n", c.Prefix()))
			ch.Part(c)
		}
	}
	s.channelsMu.RUnlock()

	c.Close()
}

func (s *Server) JoinChannel(c *client.Client, name string) {
	s.channelsMu.Lock()
	ch, exists := s.channels[strings.ToLower(name)]
	if !exists {
		ch = channel.New(name)
		s.channels[strings.ToLower(name)] = ch
	}
	s.channelsMu.Unlock()

	ch.Join(c)

	// Notify all members (including joiner)
	ch.BroadcastAll(fmt.Sprintf(":%s JOIN %s\r\n", c.Prefix(), name))

	// Send topic
	topic := ch.Topic()
	if topic != "" {
		c.SendLine(fmt.Sprintf(":%s 332 %s %s :%s\r\n", s.cfg.Server.Name, c.Nick(), name, topic))
	}

	// Send names list
	members := ch.Members()
	nicks := make([]string, len(members))
	for i, m := range members {
		nicks[i] = m.Nick()
	}
	c.SendLine(fmt.Sprintf(":%s 353 %s = %s :%s\r\n", s.cfg.Server.Name, c.Nick(), name, strings.Join(nicks, " ")))
	c.SendLine(fmt.Sprintf(":%s 366 %s %s :End of /NAMES list\r\n", s.cfg.Server.Name, c.Nick(), name))
}

func (s *Server) PartChannel(c *client.Client, name string, reason string) {
	s.channelsMu.RLock()
	ch, exists := s.channels[strings.ToLower(name)]
	s.channelsMu.RUnlock()

	if !exists || !ch.HasMember(c) {
		c.SendLine(fmt.Sprintf(":%s 442 %s %s :You're not on that channel\r\n",
			s.cfg.Server.Name, c.Nick(), name))
		return
	}

	partMsg := fmt.Sprintf(":%s PART %s", c.Prefix(), name)
	if reason != "" {
		partMsg += " :" + reason
	}
	partMsg += "\r\n"

	ch.BroadcastAll(partMsg)
	ch.Part(c)

	// Clean up empty channels
	if ch.MemberCount() == 0 {
		s.channelsMu.Lock()
		delete(s.channels, strings.ToLower(name))
		s.channelsMu.Unlock()
	}
}

func (s *Server) ChannelNames(name string) []string {
	s.channelsMu.RLock()
	ch, exists := s.channels[strings.ToLower(name)]
	s.channelsMu.RUnlock()

	if !exists {
		return nil
	}

	members := ch.Members()
	nicks := make([]string, len(members))
	for i, m := range members {
		nicks[i] = m.Nick()
	}
	return nicks
}

func (s *Server) ChannelTopic(name string) string {
	s.channelsMu.RLock()
	ch, exists := s.channels[strings.ToLower(name)]
	s.channelsMu.RUnlock()
	if !exists {
		return ""
	}
	return ch.Topic()
}

func (s *Server) SetChannelTopic(c *client.Client, name string, topic string) {
	s.channelsMu.RLock()
	ch, exists := s.channels[strings.ToLower(name)]
	s.channelsMu.RUnlock()

	if !exists || !ch.HasMember(c) {
		c.SendLine(fmt.Sprintf(":%s 442 %s %s :You're not on that channel\r\n",
			s.cfg.Server.Name, c.Nick(), name))
		return
	}

	ch.SetTopic(topic)
	ch.BroadcastAll(fmt.Sprintf(":%s TOPIC %s :%s\r\n", c.Prefix(), name, topic))
}

func (s *Server) ListChannels() []command.ChannelInfo {
	s.channelsMu.RLock()
	defer s.channelsMu.RUnlock()

	list := make([]command.ChannelInfo, 0, len(s.channels))
	for _, ch := range s.channels {
		list = append(list, command.ChannelInfo{
			Name:        ch.Name(),
			MemberCount: ch.MemberCount(),
			Topic:       ch.Topic(),
		})
	}
	return list
}

func (s *Server) BroadcastToChannel(sender *client.Client, channelName string, line string) bool {
	s.channelsMu.RLock()
	ch, exists := s.channels[strings.ToLower(channelName)]
	s.channelsMu.RUnlock()

	if !exists {
		return false
	}

	ch.Broadcast(sender, line)
	return true
}

func (s *Server) logError(msg string, err error) {
	if s.log != nil {
		s.log.Error(msg, infralogger.Error(err))
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd ircd && go test ./internal/server/ -v -timeout 30s`
Expected: All 4 integration tests PASS.

- [ ] **Step 5: Run all tests**

Run: `cd ircd && go test ./... -v`
Expected: All tests across all packages PASS.

- [ ] **Step 6: Commit**

```bash
git add ircd/internal/server/
git commit -m "feat(ircd): add server package with TCP listener and command dispatch"
```

---

### Task 11: Wire Up main.go

**Files:**
- Modify: `ircd/main.go`

- [ ] **Step 1: Update main.go to bootstrap the server**

```go
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/ircd/internal/config"
	"github.com/jonesrussell/north-cloud/ircd/internal/server"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("NorthCloud IRCd %s\n", version)
		os.Exit(0)
	}

	// Load config
	cfgPath := "config.yml"
	if envPath := os.Getenv("IRCD_CONFIG"); envPath != "" {
		cfgPath = envPath
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Setup logger
	log, err := infralogger.New(infralogger.Config{
		Level:  "info",
		Format: "json",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = log.Sync() }()
	log = log.With(infralogger.String("service", "ircd"))

	// Start server
	srv := server.New(cfg, log)
	addr, err := srv.Start()
	if err != nil {
		log.Error("Failed to start server", infralogger.Error(err))
		os.Exit(1)
	}

	log.Info("IRCd started",
		infralogger.String("version", version),
		infralogger.String("address", addr),
		infralogger.String("server_name", cfg.Server.Name),
		infralogger.String("network", cfg.Server.Network),
	)

	// Wait for shutdown signal
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	sig := <-shutdown
	log.Info("Shutdown signal received", infralogger.String("signal", sig.String()))

	srv.Shutdown()
	log.Info("IRCd stopped")
}
```

- [ ] **Step 2: Create a test config.yml for manual testing**

Copy `config.yml.example` to `config.yml` (gitignored).

- [ ] **Step 3: Build and verify startup**

Run: `cd ircd && go build -o bin/ircd . && ./bin/ircd`
Expected: Server starts, logs JSON output with address, waits for connections. Ctrl+C stops it.

- [ ] **Step 4: Manual smoke test with netcat or an IRC client**

```bash
# In one terminal:
cd ircd && ./bin/ircd

# In another terminal:
nc 127.0.0.1 6667
NICK testuser
USER testuser 0 * :Test User
# Should see welcome burst (001-005, MOTD)
JOIN #test
PRIVMSG #test :hello world
QUIT
```

- [ ] **Step 5: Commit**

```bash
git add ircd/main.go
git commit -m "feat(ircd): wire up main.go with config, logging, and signal handling"
```

---

### Task 12: Add .gitignore and Documentation

**Files:**
- Create: `ircd/.gitignore`
- Create: `ircd/CLAUDE.md`

- [ ] **Step 1: Create .gitignore**

```
bin/
config.yml
coverage.out
coverage.html
```

- [ ] **Step 2: Create CLAUDE.md**

```markdown
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
```

- [ ] **Step 3: Commit**

```bash
git add ircd/.gitignore ircd/CLAUDE.md
git commit -m "docs(ircd): add .gitignore and CLAUDE.md"
```

---

### Task 13: Final Integration Test & Race Detection

**Files:**
- No new files — verification only

- [ ] **Step 1: Run full test suite**

Run: `cd ircd && go test ./... -v`
Expected: All tests PASS.

- [ ] **Step 2: Run with race detector**

Run: `cd ircd && go test -race ./... -v -timeout 60s`
Expected: No race conditions detected.

- [ ] **Step 3: Run linter**

Run: `cd ircd && go vet ./... && go fmt ./...`
Expected: No issues.

- [ ] **Step 4: Build final binary**

Run: `cd ircd && task build`
Expected: Clean build.

- [ ] **Step 5: Commit any fixes from above steps**

Only if race detector or linter found issues.
