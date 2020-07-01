package bridge

import (
	"log"
	"strings"
	"time"
)

// findAvatar tries to find the avatar of a nick on a server.
// It needs to list members of the guild first, cached for 5 min.
func (b *Bot) findAvatar(guildID string, nick string) string {
	nick = strings.ToLower(nick)

	b.m.Lock()
	avatars := b.avatars
	b.m.Unlock()
	if avatars != nil {
		return b.avatars[nick]
	}

	const after = ""
	const limit = 1000
	members, err := b.session.GuildMembers(guildID, after, limit)
	if err != nil {
		log.Printf("error resolving guild members %v: %v", guildID, err)
		return nick
	}
	avatars = map[string]string{}
	for _, m := range members {
		n := m.User.Username
		if m.Nick != "" {
			n = m.Nick // user specified a nickname for this server
		}
		avatars[strings.ToLower(n)] = m.User.AvatarURL("128")
	}

	b.m.Lock()
	b.avatars = avatars
	b.m.Unlock()

	go func() {
		<-time.After(5 * time.Minute)
		b.m.Lock()
		b.avatars = nil
		b.m.Unlock()
	}()

	return avatars[nick]
}
