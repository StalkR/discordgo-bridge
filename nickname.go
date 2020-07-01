package bridge

import (
  "log"
  "time"

  "github.com/bwmarrin/discordgo"
)

// resolveNickname finds the nickname for a given user.
// User can specify a nickname on each server, different from their username.
// The result is cached for 1 minute.
func (b *Bot) resolveNickname(guildID string, author *discordgo.User) string {
  b.m.Lock()
  nick, ok := b.users[author.ID]
  b.m.Unlock()
  if ok {
    return nick
  }
  nick = author.Username
  member, err := b.session.GuildMember(guildID, author.ID)
  if err != nil {
    log.Printf("error resolving guild member %v (%v): %v", author.Username, author.ID, err)
    return nick
  }
  if member.Nick != "" {
    nick = member.Nick // user configured a nickname on this server
  }
  b.m.Lock()
  b.users[author.ID] = nick
  b.m.Unlock()
  go func(id string) {
    <-time.After(time.Minute)
    b.m.Lock()
    delete(b.users, id)
    b.m.Unlock()
  }(author.ID)
  return nick
}
