package command

import (
	"strings"

	"github.com/jonesrussell/north-cloud/ircd/internal/client"
	"github.com/jonesrussell/north-cloud/ircd/internal/message"
)

// ServerInterface defines what command handlers need from the server.
// Avoids circular imports between command and server packages.
type ServerInterface interface {
	ServerName() string
	NetworkName() string
	MOTD() string
	ClientCount() int

	// Client operations
	FindClientByNick(nick string) *client.Client
	UnregisterClient(c *client.Client)
	ChangeNick(c *client.Client, newNick string) error

	// Channel operations
	JoinChannel(c *client.Client, name string)
	PartChannel(c *client.Client, name string, reason string)
	ChannelNames(name string) []string
	ChannelTopic(name string) string
	SetChannelTopic(c *client.Client, name string, topic string)
	ListChannels() []ChannelInfo
	BroadcastToChannel(sender *client.Client, channelName string, line string) bool
}

// ChannelInfo holds summary info for LIST replies.
type ChannelInfo struct {
	Name        string
	MemberCount int
	Topic       string
}

// Context is passed to every command handler.
type Context struct {
	Server  ServerInterface
	Client  *client.Client
	Message *message.Message
}

// HandlerFunc is the signature for command handlers.
type HandlerFunc func(ctx *Context)

// Registry maps IRC commands to handler functions.
type Registry struct {
	handlers map[string]HandlerFunc
}

func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[string]HandlerFunc),
	}
}

func (r *Registry) Register(command string, handler HandlerFunc) {
	r.handlers[strings.ToUpper(command)] = handler
}

func (r *Registry) Lookup(command string) (HandlerFunc, bool) {
	h, ok := r.handlers[strings.ToUpper(command)]
	return h, ok
}
