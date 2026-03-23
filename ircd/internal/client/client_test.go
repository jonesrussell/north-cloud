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
	assert.Contains(t, c.Prefix(), "jones!jones@")
}
