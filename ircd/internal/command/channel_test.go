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
	assert.Contains(t, line, "461")
}

func TestHandleJoin_InvalidChannel(t *testing.T) {
	srv := newMockServer()
	ctx, conn := newTestCtx(t, srv)
	ctx.Client.SetNick("jones")
	ctx.Client.SetUser("jones", "Jones")
	ctx.Message = &message.Message{Command: "JOIN", Params: []string{"nochanprefix"}}

	command.HandleJoin(ctx)

	line := readLine(t, conn)
	assert.Contains(t, line, "403")
}

func TestHandlePart_NoParams(t *testing.T) {
	srv := newMockServer()
	ctx, conn := newTestCtx(t, srv)
	ctx.Client.SetNick("jones")
	ctx.Client.SetUser("jones", "Jones")
	ctx.Message = &message.Message{Command: "PART"}

	command.HandlePart(ctx)

	line := readLine(t, conn)
	assert.Contains(t, line, "461")
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

	line := readLine(t, conn)
	assert.Contains(t, line, "321")
}
