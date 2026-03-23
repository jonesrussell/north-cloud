package message

import (
	"errors"
	"strings"
)

var ErrEmptyMessage = errors.New("empty message")

// Message represents a parsed IRC protocol message.
type Message struct {
	Prefix  string
	Command string
	Params  []string
}

// Parse parses a raw IRC line into a Message.
func Parse(raw string) (*Message, error) {
	raw = strings.TrimRight(raw, "\r\n")
	if raw == "" {
		return nil, ErrEmptyMessage
	}

	msg := &Message{}
	s := raw

	// Parse optional prefix
	if strings.HasPrefix(s, ":") {
		idx := strings.Index(s, " ")
		if idx == -1 {
			return nil, ErrEmptyMessage
		}
		msg.Prefix = s[1:idx]
		s = s[idx+1:]
	}

	// Parse command
	if idx := strings.Index(s, " "); idx != -1 {
		msg.Command = strings.ToUpper(s[:idx])
		s = s[idx+1:]
	} else {
		msg.Command = strings.ToUpper(s)
		return msg, nil
	}

	// Parse params
	for s != "" {
		if strings.HasPrefix(s, ":") {
			msg.Params = append(msg.Params, s[1:])
			break
		}
		if idx := strings.Index(s, " "); idx != -1 {
			msg.Params = append(msg.Params, s[:idx])
			s = s[idx+1:]
		} else {
			msg.Params = append(msg.Params, s)
			break
		}
	}

	return msg, nil
}

// String formats the message back to IRC wire format.
func (m *Message) String() string {
	var b strings.Builder

	if m.Prefix != "" {
		b.WriteByte(':')
		b.WriteString(m.Prefix)
		b.WriteByte(' ')
	}

	b.WriteString(m.Command)

	for i, p := range m.Params {
		b.WriteByte(' ')
		if i == len(m.Params)-1 && (strings.Contains(p, " ") || p == "" || strings.HasPrefix(p, ":")) {
			b.WriteByte(':')
		}
		b.WriteString(p)
	}

	b.WriteString("\r\n")
	return b.String()
}
