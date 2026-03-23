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
