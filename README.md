# Discord Go Bridge library

[![Build Status][1]][2] [![Godoc][3]][4]

A small library using [discordgo][5] to implement bridges between discord and
other chats (e.g. IRC).
On the discord side, the bot uses webhook to impersonate nicknames, and if the
nickname matches one on discord, also uses their avatar.

## Example
```
import (
  bridge "github.com/StalkR/discordgo-bridge"
)

general := bridge.NewChannel("#general", "https://discord.com/api/webhooks/<webhook id>/<webhook token>", func(nick, text string) {
  log.Printf("relaying from discord -> external: <%v> %v", nick, text)
  // implement sending to external
})

go func() {
  // implement receiving from external
  for ; ; time.Sleep(10 * time.Second) {
    nick, text := "timebot", fmt.Sprintf("Current time: %v", time.Now())
    log.Printf("relaying from external -> discord: <%v> %v", nick, text)
    general.Send(nick, text)
  }
}()

b := bridge.NewBot("<bot token>", general)
if err := b.Start(); err != nil {
  log.Fatal(err)
}
defer b.Close()

log.Printf("Discord bridge bot running - press CTRL-C to exit")
sc := make(chan os.Signal, 1)
signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
<-sc
```

## Acknowledgements
Discord Go bindings: [bwmarrin/discordgo][5] ([doc][6]).

## Bugs, comments, questions
Create a [new issue][7].

[1]: https://github.com/StalkR/discordgo-bridge/actions/workflows/build.yml/badge.svg
[2]: https://github.com/StalkR/discordgo-bridge/actions/workflows/build.yml
[3]: https://godoc.org/github.com/StalkR/discordgo-bridge?status.png
[4]: https://godoc.org/github.com/StalkR/discordgo-bridge
[5]: https://github.com/bwmarrin/discordgo
[6]: https://godoc.org/github.com/bwmarrin/discordgo
[7]: https://github.com/StalkR/discordgo-bridge/issues/new
