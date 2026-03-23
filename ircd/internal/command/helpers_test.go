package command_test

import (
	"net"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/ircd/internal/client"
	"github.com/jonesrussell/north-cloud/ircd/internal/command"
	"github.com/stretchr/testify/require"
)

type mockServer struct {
	name        string
	network     string
	motd        string
	clients     map[string]*client.Client
	nickErr     error
	channelList []command.ChannelInfo
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
func (s *mockServer) UnregisterClient(_ *client.Client)           {}
func (s *mockServer) ChangeNick(c *client.Client, nick string) error {
	if s.nickErr != nil {
		return s.nickErr
	}
	delete(s.clients, c.Nick())
	c.SetNick(nick)
	s.clients[nick] = c
	return nil
}
func (s *mockServer) JoinChannel(_ *client.Client, _ string)                             {}
func (s *mockServer) PartChannel(_ *client.Client, _ string, _ string)                   {}
func (s *mockServer) ChannelNames(_ string) []string                                     { return nil }
func (s *mockServer) ChannelTopic(_ string) string                                       { return "" }
func (s *mockServer) SetChannelTopic(_ *client.Client, _ string, _ string)               {}
func (s *mockServer) ListChannels() []command.ChannelInfo                                { return s.channelList }
func (s *mockServer) BroadcastToChannel(_ *client.Client, _ string, _ string) bool       { return true }

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
		Server: srv,
		Client: c,
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

func timeoutDuration() time.Time {
	return time.Now().Add(50 * time.Millisecond)
}
