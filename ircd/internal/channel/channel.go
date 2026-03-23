package channel

import (
	"sync"

	"github.com/jonesrussell/north-cloud/ircd/internal/client"
)

type Channel struct {
	name    string
	topic   string
	members map[*client.Client]bool
	mu      sync.RWMutex
}

func New(name string) *Channel {
	return &Channel{
		name:    name,
		members: make(map[*client.Client]bool),
	}
}

func (ch *Channel) Name() string {
	return ch.name
}

func (ch *Channel) Topic() string {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	return ch.topic
}

func (ch *Channel) SetTopic(topic string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.topic = topic
}

func (ch *Channel) MemberCount() int {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	return len(ch.members)
}

func (ch *Channel) HasMember(c *client.Client) bool {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	return ch.members[c]
}

func (ch *Channel) Join(c *client.Client) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.members[c] = true
}

func (ch *Channel) Part(c *client.Client) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	delete(ch.members, c)
}

func (ch *Channel) Members() []*client.Client {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	members := make([]*client.Client, 0, len(ch.members))
	for c := range ch.members {
		members = append(members, c)
	}
	return members
}

func (ch *Channel) Broadcast(sender *client.Client, line string) {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	for c := range ch.members {
		if c != sender {
			c.SendLine(line)
		}
	}
}

func (ch *Channel) BroadcastAll(line string) {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	for c := range ch.members {
		c.SendLine(line)
	}
}
