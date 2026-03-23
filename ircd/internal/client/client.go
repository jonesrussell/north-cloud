package client

import (
	"bufio"
	"fmt"
	"net"
	"sync"
)

const sendBufferSize = 512

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

func (c *Client) Prefix() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	host := c.conn.RemoteAddr().String()
	return fmt.Sprintf("%s!%s@%s", c.nick, c.username, host)
}

func (c *Client) Hostname() string {
	return c.conn.RemoteAddr().String()
}

// SendLine queues a line to be sent. Returns false if client is closing or buffer full.
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
