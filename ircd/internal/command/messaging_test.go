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
	assert.Contains(t, line, "411")
}

func TestHandlePrivmsg_NoSuchNick(t *testing.T) {
	srv := newMockServer()
	ctx, conn := newTestCtx(t, srv)
	ctx.Client.SetNick("alice")
	ctx.Client.SetUser("alice", "Alice")
	ctx.Message = &message.Message{Command: "PRIVMSG", Params: []string{"nobody", "hello"}}

	command.HandlePrivmsg(ctx)

	line := readLine(t, conn)
	assert.Contains(t, line, "401")
}
