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

  token    string                     // discord API token
  channels map[string]*bridge.Channel // irc -> discord channels
  discord  *bridge.Bot

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
    channels: map[string]*bridge.Channel{},
  }

  for _, option := range options {
    option(b)
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

  b.conn.HandleFunc("connected",
    func(conn *client.Conn, line *client.Line) {
      conn.Mode(conn.Me().Nick, "+B")
      for channel := range b.channels {
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
  for _, v := range b.channels {
    list = append(list, v)
  }
  d := bridge.NewBot(b.token, list...)
  if err := d.Start(); err != nil {
    return nil, err
  }

  b.conn.HandleFunc("privmsg",
    func(conn *client.Conn, line *client.Line) { toDiscord(d, line, b.channels) })

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

func toIRC(b *Bot, channel, nick, text string) {
  if strings.HasPrefix(text, commandPrefix) {
    b.conn.Privmsg(channel, fmt.Sprintf("Command sent from discord by %v", nick))
    b.conn.Privmsg(channel, text)
    return
  }
  b.conn.Privmsg(channel, fmt.Sprintf("<%v> %v", nick, text))
}

func toDiscord(d *bridge.Bot, line *client.Line, channels map[string]*bridge.Channel) {
  channel := line.Args[0]
  nick := line.Nick
  text := line.Args[1]
  c, ok := channels[channel]
  if !ok {
    return
  }
  c.Send(nick, text)
}

// An Option configures a Bot in New.
type Option func(*Bot)

// Host configures the IRC server host:port.
func Host(v string) Option {
  return func(b *Bot) { b.config.Server = v }
}

// Nick configures the IRC nick of the bot.
func Nick(v string) Option {
  return func(b *Bot) {
    b.config.Me.Nick = v
    b.config.Me.Ident = v
    b.config.Me.Name = v
  }
}

// TLS configures whether the IRC connection should use TLS. Default is true.
func TLS(v bool) Option {
  return func(b *Bot) { b.config.SSL = v }
}

// Token configures the Discord bot token.
func Token(v string) Option {
  return func(b *Bot) { b.token = v }
}

// Channel configures IRC channel to Discord channel and webhook.
// Can be used multiple times.
// An IRC channel can only have one Discord channel configured.
func Channel(irc string, discord string, webhook string) Option {
  return func(b *Bot) {
    b.channels[irc] = bridge.NewChannel(discord, webhook, func(nick, text string) {
      toIRC(b, irc, nick, text)
    })
  }
}
