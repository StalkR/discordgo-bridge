/*
Package bridge is a library to bridge communications with discord.

  import bridge "github.com/StalkR/discordgo-bridge"

Setup:

 - create a new application: https://discordapp.com/developers/applications/me
 - get client ID, required to add bot to server below
 - click add bot
 - get token, required to configure the bot
 - add bot to server: https://discordapp.com/oauth2/authorize?&client_id=CLIENT_ID&scope=bot&permissions=0
 - for each channel configure a webhook, required to configure the channels (allows impersonation)

Multiple channels can be bridged.
The bot can only be added to one server.
To use it on more than one server, create another application and run another bot.
*/
package bridge

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
)

// A Bot represents a discord bridge bot.
type Bot struct {
	token    string
	channels []*Channel
	// channel ID resolved after, maps built for quick lookups
	channelIDs map[string]*Channel
	webhookIDs map[string]bool

	m       sync.Mutex
	users   map[string]string // cache of user ID -> nickname
	avatars map[string]string // cache of nickname -> avatar URL

	session       *discordgo.Session
	removeHandler func()
}

// NewBot creates a new discord bridge bot.
func NewBot(token string, channels ...*Channel) *Bot {
	return &Bot{
		token:    token,
		channels: channels,
		users:    map[string]string{},
	}
}

// Start starts the bot, connecting it with discord.
func (b *Bot) Start() error {
	s, err := discordgo.New("Bot " + b.token)
	if err != nil {
		return fmt.Errorf("error creating session: %v", err)
	}
	b.session = s
	if err := b.initialize(); err != nil {
		b.session.Close()
		return err
	}
	return nil
}

// initialize implements the bot initialization:
// - open connection to discord
// - resolve channel names to IDs, and keep them in a map for fast dispatch
// - keep channel webhookIDs in a map for fast lookup
// - install send hook on each channel (external->discord)
// - install the receive hook (discord->external)
func (b *Bot) initialize() error {
	if err := b.session.Open(); err != nil {
		return fmt.Errorf("error opening connection: %v,", err)
	}

	// resolve channels name to channels id
	guilds := b.guilds(b.session)
	if len(guilds) != 1 {
		return fmt.Errorf("connected to %v servers (want 1)", len(guilds))
	}
	guild := guilds[0]
	guildChannels, err := b.session.GuildChannels(guild.ID)
	if err != nil {
		return fmt.Errorf("error getting channels: %v", err)
	}
	for _, c := range b.channels {
		for _, g := range guildChannels {
			if c.name == g.Name {
				c.id = g.ID
				break
			}
		}
		if c.id == "" {
			return fmt.Errorf("server has no channel #%v", c.name)
		}
	}
	b.channelIDs = map[string]*Channel{}
	b.webhookIDs = map[string]bool{}
	for _, c := range b.channels {
		b.channelIDs[c.id] = c
		b.webhookIDs[c.webhookID] = true
	}

	// install send hook: external -> discord
	for _, c := range b.channels {
		c := c // careful with scoping
		c.m.Lock()
		c.sendhook = func(nick, text string) error {
			if strings.TrimSpace(nick) == "" || strings.TrimSpace(text) == "" {
				return nil // safety
			}
			if _, err := b.session.WebhookExecute(c.webhookID, c.webhookToken, true, &discordgo.WebhookParams{
				Username:  nick,
				Content:   text,
				AvatarURL: b.findAvatar(guild.ID, nick),
			}); err != nil {
				return fmt.Errorf("error webhook execute for #%v: %v", c.name, err)
			}
			return nil
		}
		c.m.Unlock()
	}

	// install receive hook: discord -> external
	b.removeHandler = b.session.AddHandler(b.handleMessage)

	return nil
}

// Close closes the discord bridge bot.
// Send and receive hooks are uninstalled, and the discord session closed.
// Bot can be started again.
func (b *Bot) Close() error {
	// uninstall send hooks
	for _, c := range b.channels {
		c.m.Lock()
		c.sendhook = nil
		c.m.Unlock()
	}
	// uninstall receive hook
	b.removeHandler()
	return b.session.Close()
}

// guilds returns the guilds (servers) the discord bot is connected to.
func (b *Bot) guilds(s *discordgo.Session) []*discordgo.Guild {
	s.RLock()
	defer s.RUnlock()
	s.State.RLock()
	defer s.State.RUnlock()
	return s.State.Guilds
}

// handleMessage handles messages received on discord.
// It routes them according to the channels receive hook.
func (b *Bot) handleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID || b.webhookIDs[m.Author.ID] {
		return // ignore messages from the bot itself
	}
	c, ok := b.channelIDs[m.ChannelID]
	if !ok {
		return // not enabled for this channel
	}
	nick := b.resolveNickname(m.GuildID, m.Author)
	content, err := m.ContentWithMoreMentionsReplaced(b.session)
	if err != nil {
		log.Printf("ContentWithMoreMentionsReplaced error: %v", err)
		return
	}
	if strings.TrimSpace(nick) == "" || strings.TrimSpace(content) == "" {
		return // safety
	}
	c.receive(nick, content)
}
