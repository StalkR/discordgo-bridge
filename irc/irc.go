// Package irc implements an irc-discord bridge.
package irc

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	bridge "github.com/StalkR/discordgo-bridge"
	"github.com/fluffle/goirc/client"
	"github.com/fluffle/goirc/state"
)

// A Bot represents an IRC-Discord bot.
type Bot struct {
	config       *client.Config
	conn         *client.Conn
	disconnected chan struct{}

	token   string // discord API token
	discord *bridge.Bot

	discord2irc     map[string][]*ChannelIRC
	irc2discord     map[string][]*ChannelDiscord
	discord2webhook map[string]string
	discord2bridge  map[string]*bridge.Channel

	m      sync.Mutex
	closed bool
}

// New creates a new IRC-Discord bot.
func New(options ...Option) (*Bot, error) {
	b := &Bot{
		config: &client.Config{
			Me:          &state.Nick{Nick: ""},
			NewNick:     func(s string) string { return s + "_" },
			PingFreq:    3 * time.Minute,
			QuitMessage: "I have to go.",
			Server:      "",
			SSL:         true,
			SSLConfig:   nil,
			Version:     "irc-discord",
			Recover:     (*client.Conn).LogPanic,
			SplitLen:    450,
			Proxy:       "",
			Pass:        "",
		},
		discord2irc:     map[string][]*ChannelIRC{},     // relay Discord channels to IRC channels
		irc2discord:     map[string][]*ChannelDiscord{}, // relay IRC channels to Discord channels
		discord2webhook: map[string]string{},            // webhooks
		discord2bridge:  map[string]*bridge.Channel{},   // unique handles for each Discord channel
	}

	for _, option := range options {
		if err := option(b); err != nil {
			return nil, err
		}
	}

	if b.config.Server == "" {
		return nil, fmt.Errorf("missing host")
	}

	if b.config.SSL {
		h, _, err := net.SplitHostPort(b.config.Server)
		if err != nil {
			return nil, fmt.Errorf("host must be host:port")
		}
		b.config.SSLConfig = &tls.Config{
			ServerName: h,
		}
	}

	b.conn = client.Client(b.config)

	b.conn.EnableStateTracking()

	channels := map[string]struct{}{}
	for k := range b.irc2discord {
		channels[k] = struct{}{}
	}
	for _, s := range b.discord2irc {
		for _, v := range s {
			channels[v.channel] = struct{}{}
		}
	}
	b.conn.HandleFunc("connected",
		func(conn *client.Conn, line *client.Line) {
			conn.Mode(conn.Me().Nick, "+B")
			for channel := range channels {
				conn.Join(channel)
			}
		})

	b.conn.HandleFunc("disconnected",
		func(conn *client.Conn, line *client.Line) {
			b.disconnected <- struct{}{}
		})

	b.conn.HandleFunc("error",
		func(conn *client.Conn, line *client.Line) {
			log.Println(conn.Config().Server, line.Cmd, line.Text())
		})

	var list []*bridge.Channel
	for k, webhook := range b.discord2webhook {
		k := k
		channel := bridge.NewChannel(k, webhook, func(nick, text string) {
			toIRC(b, k, nick, text)
		})
		b.discord2bridge[k] = channel
		list = append(list, channel)
	}
	b.discord = bridge.NewBot(b.token, list...)
	if err := b.discord.Start(); err != nil {
		return nil, err
	}

	b.conn.HandleFunc("privmsg",
		func(conn *client.Conn, line *client.Line) {
			toDiscord(b, line)
		})
	b.conn.HandleFunc("action",
		func(conn *client.Conn, line *client.Line) {
			toDiscord(b, line)
		})

	go b.run()

	return b, nil
}

func (b *Bot) run() {
	for !b.isClosed() {
		if err := b.conn.Connect(); err != nil {
			log.Println("Connection error:", err, "- reconnecting in 1 minute")
			time.Sleep(time.Minute)
			continue
		}
		<-b.disconnected
	}
}

// Close disconnects the bot from IRC and Discord.
func (b *Bot) Close() error {
	b.m.Lock()
	defer b.m.Unlock()
	b.closed = true
	b.conn.Quit()
	return b.discord.Close()
}

func (b *Bot) isClosed() bool {
	b.m.Lock()
	defer b.m.Unlock()
	return b.closed
}

const commandPrefix = "!"

func toIRC(b *Bot, discord, nick, text string) {
	text = strings.ReplaceAll(text, "\n", "; ")
	for _, v := range b.discord2irc[discord] {
		if strings.HasPrefix(text, commandPrefix) {
			b.conn.Privmsg(v.channel, fmt.Sprintf("Command sent from discord by %v", nick))
			b.conn.Privmsg(v.channel, text)
			continue
		}
		b.conn.Privmsg(v.channel, fmt.Sprintf("<%v> %v", nick, text))
	}
}

func toDiscord(b *Bot, line *client.Line) {
	channel := line.Args[0]
	nick := line.Nick
	text := line.Args[1]
	if strings.ToLower(line.Cmd) == "action" {
		text = "/me " + text
	}
	for _, v := range b.irc2discord[channel] {
		b.discord2bridge[v.channel].Send(nick, text)
	}
}

// An Option configures a Bot in New.
type Option func(*Bot) error

// Host configures the IRC server host:port.
func Host(v string) Option {
	return func(b *Bot) error {
		b.config.Server = v
		return nil
	}
}

// Nick configures the IRC nick of the bot.
func Nick(v string) Option {
	return func(b *Bot) error {
		b.config.Me.Nick = v
		b.config.Me.Ident = v
		b.config.Me.Name = v
		return nil
	}
}

// TLS configures whether the IRC connection should use TLS. Default is true.
func TLS(v bool) Option {
	return func(b *Bot) error {
		b.config.SSL = v
		return nil
	}
}

// Token configures the Discord bot token.
func Token(v string) Option {
	return func(b *Bot) error {
		b.token = v
		return nil
	}
}

// Relay configures a relay from either IRC channel or Discord channel to the other.
// To have it both ways, use twice or use the Sync shortcut.
func Relay(from, to interface{}) Option {
	return func(b *Bot) error {
		switch v := from.(type) {
		case *ChannelIRC:
			irc := v
			discord, ok := to.(*ChannelDiscord)
			if !ok {
				return fmt.Errorf("%v is not a discord channel", to)
			}
			b.irc2discord[irc.channel] = append(b.irc2discord[irc.channel], discord)
			b.discord2webhook[discord.channel] = discord.webhook
			return nil

		case *ChannelDiscord:
			discord := v
			irc, ok := to.(*ChannelIRC)
			if !ok {
				return fmt.Errorf("%v is not an IRC channel", to)
			}
			b.discord2irc[discord.channel] = append(b.discord2irc[discord.channel], irc)
			b.discord2webhook[discord.channel] = discord.webhook
			return nil
		}
		return fmt.Errorf("%v is neither a Discord or IRC channel", from)
	}
}

// Sync an IRC channel with a Discord channel.
func Sync(irc *ChannelIRC, discord *ChannelDiscord) Option {
	return func(b *Bot) error {
		if err := Relay(irc, discord)(b); err != nil {
			return err
		}
		return Relay(discord, irc)(b)
	}
}

// ChannelIRC represents a configured IRC channel with Channel().
type ChannelIRC struct {
	channel string
}

// Channel configures an IRC channel.
func Channel(channel string) *ChannelIRC {
	return &ChannelIRC{channel: channel}
}

// ChannelDiscord represents a configured Discord channel with Discord().
type ChannelDiscord struct {
	channel string
	webhook string
}

// Discord configures a Discord channel with a webhook.
func Discord(channel string, webhook string) *ChannelDiscord {
	return &ChannelDiscord{channel: channel, webhook: webhook}
}
