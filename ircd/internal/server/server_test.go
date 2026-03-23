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
			Listen:       "127.0.0.1:0",
			MaxClients:   10,
			PingInterval: 5 * time.Minute,
			PongTimeout:  5 * time.Minute,
			MOTD:         "Welcome!",
		},
	}
	srv := server.New(cfg, nil)
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

	scanner := bufio.NewScanner(conn)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	for scanner.Scan() {
		line := scanner.Text()
		// 251 is the last line of the welcome burst (sent after 376/MOTD).
		// 422 means no MOTD (also terminal). Drain until we see one of these.
		if strings.Contains(line, "251") || strings.Contains(line, "422") {
			break
		}
	}
	conn.SetReadDeadline(time.Time{})
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
	alice.SetReadDeadline(time.Now().Add(time.Second))
	ascanner := bufio.NewScanner(alice)
	for ascanner.Scan() {
		if strings.Contains(ascanner.Text(), "366") {
			break
		}
	}
	alice.SetReadDeadline(time.Time{})

	fmt.Fprintf(bob, "JOIN #test\r\n")
	bob.SetReadDeadline(time.Now().Add(time.Second))
	bscanner := bufio.NewScanner(bob)
	for bscanner.Scan() {
		if strings.Contains(bscanner.Text(), "366") {
			break
		}
	}
	bob.SetReadDeadline(time.Time{})

	fmt.Fprintf(alice, "PRIVMSG #test :hello channel\r\n")

	bob.SetReadDeadline(time.Now().Add(2 * time.Second))
	require.True(t, bscanner.Scan())
	assert.Contains(t, bscanner.Text(), "PRIVMSG #test :hello channel")
}

func TestServer_DuplicateNick(t *testing.T) {
	addr, stop := startTestServer(t)
	defer stop()

	connectAndRegister(t, addr, "jones")

	conn, err := net.DialTimeout("tcp", addr, time.Second)
	require.NoError(t, err)
	defer conn.Close()

	fmt.Fprintf(conn, "NICK jones\r\n")

	conn.SetReadDeadline(time.Now().Add(time.Second))
	scanner := bufio.NewScanner(conn)
	require.True(t, scanner.Scan())
	assert.Contains(t, scanner.Text(), "433")
}
