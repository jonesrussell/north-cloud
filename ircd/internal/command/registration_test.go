package command_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/ircd/internal/command"
	"github.com/jonesrussell/north-cloud/ircd/internal/message"
	"github.com/stretchr/testify/assert"
)

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
	assert.Contains(t, line, "431")
}

func TestHandleNick_InvalidNick(t *testing.T) {
	srv := newMockServer()
	ctx, conn := newTestCtx(t, srv)
	ctx.Message = &message.Message{Command: "NICK", Params: []string{"1invalid"}}

	command.HandleNick(ctx)

	line := readLine(t, conn)
	assert.Contains(t, line, "432")
}

func TestHandleNick_NickInUse(t *testing.T) {
	srv := newMockServer()
	ctx1, _ := newTestCtx(t, srv)
	ctx1.Message = &message.Message{Command: "NICK", Params: []string{"jones"}}
	command.HandleNick(ctx1)

	ctx2, conn2 := newTestCtx(t, srv)
	ctx2.Message = &message.Message{Command: "NICK", Params: []string{"jones"}}
	command.HandleNick(ctx2)

	line := readLine(t, conn2)
	assert.Contains(t, line, "433")
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

func TestHandlePing_NoParams(t *testing.T) {
	srv := newMockServer()
	ctx, conn := newTestCtx(t, srv)
	ctx.Message = &message.Message{Command: "PING"}

	command.HandlePing(ctx)

	line := readLine(t, conn)
	assert.Contains(t, line, "PONG")
	assert.Contains(t, line, srv.ServerName())
}

func TestHandleUser_SetsUser(t *testing.T) {
	srv := newMockServer()
	ctx, _ := newTestCtx(t, srv)
	ctx.Message = &message.Message{
		Command: "USER",
		Params:  []string{"jones", "0", "*", "Russell Jones"},
	}

	command.HandleUser(ctx)
	assert.Equal(t, "jones", ctx.Client.Username())
	assert.Equal(t, "Russell Jones", ctx.Client.Realname())
}

func TestHandleUser_NotEnoughParams(t *testing.T) {
	srv := newMockServer()
	ctx, conn := newTestCtx(t, srv)
	ctx.Message = &message.Message{Command: "USER", Params: []string{"jones"}}

	command.HandleUser(ctx)

	line := readLine(t, conn)
	assert.Contains(t, line, "461")
}

func TestHandleUser_AlreadyRegistered(t *testing.T) {
	srv := newMockServer()
	ctx, conn := newTestCtx(t, srv)

	// Register first
	ctx.Message = &message.Message{Command: "NICK", Params: []string{"jones"}}
	command.HandleNick(ctx)
	ctx.Message = &message.Message{Command: "USER", Params: []string{"jones", "0", "*", "Russell Jones"}}
	command.HandleUser(ctx)

	// Drain welcome messages
	for {
		buf := make([]byte, 4096)
		conn.SetReadDeadline(timeoutDuration())
		n, err := conn.Read(buf)
		if err != nil || n == 0 {
			break
		}
		// Keep reading until timeout
	}

	// Try to re-register
	ctx.Message = &message.Message{Command: "USER", Params: []string{"other", "0", "*", "Other User"}}
	command.HandleUser(ctx)

	line := readLine(t, conn)
	assert.Contains(t, line, "462")
}

func TestHandleQuit(t *testing.T) {
	srv := newMockServer()
	ctx, conn := newTestCtx(t, srv)
	ctx.Message = &message.Message{Command: "QUIT", Params: []string{"bye"}}

	command.HandleQuit(ctx)

	line := readLine(t, conn)
	assert.Contains(t, line, "ERROR")
	assert.Contains(t, line, "bye")
}

func TestHandlePong(t *testing.T) {
	srv := newMockServer()
	ctx, _ := newTestCtx(t, srv)
	ctx.Message = &message.Message{Command: "PONG", Params: []string{"test.irc"}}
	// HandlePong is a no-op; just ensure it doesn't panic
	command.HandlePong(ctx)
}

func TestHandleNick_SendsWelcomeWhenFullyRegistered(t *testing.T) {
	srv := newMockServer()
	ctx, conn := newTestCtx(t, srv)

	// Set USER first (no welcome yet)
	ctx.Message = &message.Message{Command: "USER", Params: []string{"jones", "0", "*", "Russell Jones"}}
	command.HandleUser(ctx)

	// Now NICK should trigger welcome
	ctx.Message = &message.Message{Command: "NICK", Params: []string{"jones"}}
	command.HandleNick(ctx)

	line := readLine(t, conn)
	assert.Contains(t, line, "001")
}
