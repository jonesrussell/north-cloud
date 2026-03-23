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
