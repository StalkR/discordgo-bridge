package bridge

import (
	"strings"

	"github.com/bwmarrin/discordgo"
)

// findAvatarURL tries to find the avatar of a nick on a server.
func findAvatarURL(guild *discordgo.Guild, nick string) string {
	avatarURLs := map[string]string{}
	for _, m := range guild.Members {
		n := m.User.Username
		if m.Nick != "" {
			n = m.Nick // user specified a nickname for this server
		}
		avatarURLs[strings.ToLower(n)] = m.User.AvatarURL("128")
	}
	return avatarURLs[strings.ToLower(nick)]
}
