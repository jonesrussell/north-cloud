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

	ch.Broadcast(c1, ":alice PRIVMSG #test :hello\r\n")

	buf := make([]byte, 512)
	s2.SetReadDeadline(time.Now().Add(time.Second))
	n, err := s2.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, ":alice PRIVMSG #test :hello\r\n", string(buf[:n]))

	s1.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	_, err = s1.Read(buf)
	assert.Error(t, err)
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
