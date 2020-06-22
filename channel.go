package bridge

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

// A Channel represents a discord channel bridge.
type Channel struct {
	name         string
	id           string // resolved from name at startup
	webhookID    string
	webhookToken string

	// user-provided function is called to receive from discord -> external
	receive func(nick, text string)

	m sync.Mutex // protects sendhook
	// user calls Send to send from external -> discord
	// hook is installed during startup once connection is established
	sendhook func(nick, text string) error
}

// NewChannel creates a new channel bridge.
// The receive function is called when messages are received on discord.
func NewChannel(name, webhookURL string, receive func(nick, text string)) *Channel {
	parts := strings.Split(webhookURL, "/")
	if len(parts) != 7 {
		panic(fmt.Errorf("invalid webhook URL for %v: %v", name, webhookURL))
	}
	return &Channel{
		name:         strings.TrimPrefix(name, "#"),
		webhookID:    parts[5],
		webhookToken: parts[6],
		receive:      receive,
	}
}

// Send sends a message to discord.
// It returns ErrNotConnected if called when bot is not connected to discord.
func (c *Channel) Send(nick, text string) error {
	c.m.Lock()
	send := c.sendhook
	c.m.Unlock()
	if send == nil {
		return ErrNotConnected
	}
	return send(nick, text)
}

// ErrNotConnected is returned when sending but the bot is not connected.
var ErrNotConnected = errors.New("not connected")
