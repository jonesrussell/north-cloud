package server

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/ircd/internal/channel"
	"github.com/jonesrussell/north-cloud/ircd/internal/client"
	"github.com/jonesrussell/north-cloud/ircd/internal/command"
	"github.com/jonesrussell/north-cloud/ircd/internal/config"
	"github.com/jonesrussell/north-cloud/ircd/internal/message"
)

// Server implements command.ServerInterface and manages the IRC daemon.
type Server struct {
	cfg      *config.Config
	log      infralogger.Logger
	listener net.Listener
	registry *command.Registry

	clients    map[string]*client.Client
	clientsMu  sync.RWMutex
	channels   map[string]*channel.Channel
	channelsMu sync.RWMutex

	quit chan struct{}
	wg   sync.WaitGroup
}

// New creates a new Server. log may be nil (logging is skipped).
func New(cfg *config.Config, log infralogger.Logger) *Server {
	s := &Server{
		cfg:      cfg,
		log:      log,
		clients:  make(map[string]*client.Client),
		channels: make(map[string]*channel.Channel),
		quit:     make(chan struct{}),
	}

	s.registry = command.NewRegistry()
	s.registry.Register("PASS", command.HandlePass)
	s.registry.Register("NICK", command.HandleNick)
	s.registry.Register("USER", command.HandleUser)
	s.registry.Register("QUIT", command.HandleQuit)
	s.registry.Register("PING", command.HandlePing)
	s.registry.Register("PONG", command.HandlePong)
	s.registry.Register("PRIVMSG", command.HandlePrivmsg)
	s.registry.Register("NOTICE", command.HandleNotice)
	s.registry.Register("JOIN", command.HandleJoin)
	s.registry.Register("PART", command.HandlePart)
	s.registry.Register("TOPIC", command.HandleTopic)
	s.registry.Register("NAMES", command.HandleNames)
	s.registry.Register("LIST", command.HandleList)

	return s
}

// Start binds the TCP listener and begins accepting connections.
// Returns the bound address (useful when Listen is "host:0").
func (s *Server) Start() (string, error) {
	ln, err := net.Listen("tcp", s.cfg.Server.Listen)
	if err != nil {
		return "", fmt.Errorf("listen: %w", err)
	}
	s.listener = ln
	go s.acceptLoop()
	return ln.Addr().String(), nil
}

// Shutdown signals all goroutines to stop, disconnects clients, and waits.
func (s *Server) Shutdown() {
	close(s.quit)
	s.listener.Close()

	s.clientsMu.RLock()
	for _, c := range s.clients {
		c.SendLine("ERROR :Server shutting down\r\n")
		c.Close()
	}
	s.clientsMu.RUnlock()

	s.wg.Wait()
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.quit:
				return
			default:
				s.logError("accept error", err)
				continue
			}
		}
		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer s.wg.Done()

	c := client.New(conn)
	go c.StartWriteLoop()

	scanner := c.ReadLines()
	for scanner.Scan() {
		raw := scanner.Text()
		if raw == "" {
			continue
		}
		if len(raw) > 510 {
			raw = raw[:510]
		}

		msg, err := message.Parse(raw)
		if err != nil {
			continue
		}

		handler, ok := s.registry.Lookup(msg.Command)
		if !ok {
			if c.Registered() {
				c.SendLine(fmt.Sprintf(":%s 421 %s %s :Unknown command\r\n",
					s.cfg.Server.Name, c.Nick(), msg.Command))
			}
			continue
		}

		ctx := &command.Context{
			Server:  s,
			Client:  c,
			Message: msg,
		}
		handler(ctx)
	}

	s.UnregisterClient(c)
}

// --- ServerInterface implementation ---

func (s *Server) ServerName() string  { return s.cfg.Server.Name }
func (s *Server) NetworkName() string { return s.cfg.Server.Network }
func (s *Server) MOTD() string        { return s.cfg.Server.MOTD }

func (s *Server) ClientCount() int {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()
	return len(s.clients)
}

func (s *Server) FindClientByNick(nick string) *client.Client {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()
	return s.clients[strings.ToLower(nick)]
}

func (s *Server) ChangeNick(c *client.Client, newNick string) error {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	lower := strings.ToLower(newNick)
	if existing, exists := s.clients[lower]; exists && existing != c {
		return errors.New("nick in use")
	}

	oldNick := strings.ToLower(c.Nick())
	if oldNick != "" {
		delete(s.clients, oldNick)
	}

	c.SetNick(newNick)
	s.clients[lower] = c
	return nil
}

func (s *Server) UnregisterClient(c *client.Client) {
	s.clientsMu.Lock()
	nick := strings.ToLower(c.Nick())
	delete(s.clients, nick)
	s.clientsMu.Unlock()

	s.channelsMu.RLock()
	for _, ch := range s.channels {
		if ch.HasMember(c) {
			ch.BroadcastAll(fmt.Sprintf(":%s QUIT :Client quit\r\n", c.Prefix()))
			ch.Part(c)
		}
	}
	s.channelsMu.RUnlock()

	c.Close()
}

func (s *Server) JoinChannel(c *client.Client, name string) {
	s.channelsMu.Lock()
	ch, exists := s.channels[strings.ToLower(name)]
	if !exists {
		ch = channel.New(name)
		s.channels[strings.ToLower(name)] = ch
	}
	s.channelsMu.Unlock()

	ch.Join(c)
	ch.BroadcastAll(fmt.Sprintf(":%s JOIN %s\r\n", c.Prefix(), name))

	topic := ch.Topic()
	if topic != "" {
		c.SendLine(fmt.Sprintf(":%s 332 %s %s :%s\r\n", s.cfg.Server.Name, c.Nick(), name, topic))
	}

	members := ch.Members()
	nicks := make([]string, len(members))
	for i, m := range members {
		nicks[i] = m.Nick()
	}
	c.SendLine(fmt.Sprintf(":%s 353 %s = %s :%s\r\n", s.cfg.Server.Name, c.Nick(), name, strings.Join(nicks, " ")))
	c.SendLine(fmt.Sprintf(":%s 366 %s %s :End of /NAMES list\r\n", s.cfg.Server.Name, c.Nick(), name))
}

func (s *Server) PartChannel(c *client.Client, name string, reason string) {
	s.channelsMu.RLock()
	ch, exists := s.channels[strings.ToLower(name)]
	s.channelsMu.RUnlock()

	if !exists || !ch.HasMember(c) {
		c.SendLine(fmt.Sprintf(":%s 442 %s %s :You're not on that channel\r\n",
			s.cfg.Server.Name, c.Nick(), name))
		return
	}

	partMsg := fmt.Sprintf(":%s PART %s", c.Prefix(), name)
	if reason != "" {
		partMsg += " :" + reason
	}
	partMsg += "\r\n"

	ch.BroadcastAll(partMsg)
	ch.Part(c)

	if ch.MemberCount() == 0 {
		s.channelsMu.Lock()
		delete(s.channels, strings.ToLower(name))
		s.channelsMu.Unlock()
	}
}

func (s *Server) ChannelNames(name string) []string {
	s.channelsMu.RLock()
	ch, exists := s.channels[strings.ToLower(name)]
	s.channelsMu.RUnlock()
	if !exists {
		return nil
	}
	members := ch.Members()
	nicks := make([]string, len(members))
	for i, m := range members {
		nicks[i] = m.Nick()
	}
	return nicks
}

func (s *Server) ChannelTopic(name string) string {
	s.channelsMu.RLock()
	ch, exists := s.channels[strings.ToLower(name)]
	s.channelsMu.RUnlock()
	if !exists {
		return ""
	}
	return ch.Topic()
}

func (s *Server) SetChannelTopic(c *client.Client, name string, topic string) {
	s.channelsMu.RLock()
	ch, exists := s.channels[strings.ToLower(name)]
	s.channelsMu.RUnlock()

	if !exists || !ch.HasMember(c) {
		c.SendLine(fmt.Sprintf(":%s 442 %s %s :You're not on that channel\r\n",
			s.cfg.Server.Name, c.Nick(), name))
		return
	}

	ch.SetTopic(topic)
	ch.BroadcastAll(fmt.Sprintf(":%s TOPIC %s :%s\r\n", c.Prefix(), name, topic))
}

func (s *Server) ListChannels() []command.ChannelInfo {
	s.channelsMu.RLock()
	defer s.channelsMu.RUnlock()
	list := make([]command.ChannelInfo, 0, len(s.channels))
	for _, ch := range s.channels {
		list = append(list, command.ChannelInfo{
			Name:        ch.Name(),
			MemberCount: ch.MemberCount(),
			Topic:       ch.Topic(),
		})
	}
	return list
}

func (s *Server) BroadcastToChannel(sender *client.Client, channelName string, line string) bool {
	s.channelsMu.RLock()
	ch, exists := s.channels[strings.ToLower(channelName)]
	s.channelsMu.RUnlock()
	if !exists {
		return false
	}
	ch.Broadcast(sender, line)
	return true
}

func (s *Server) logError(msg string, err error) {
	if s.log != nil {
		s.log.Error(msg, infralogger.Error(err))
	}
}
