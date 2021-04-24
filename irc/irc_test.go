package irc

import (
  "log"
  "os"
  "os/signal"
  "syscall"
)

func ExampleBot() {
  b, err := New(
    Host("irc.example.com:6697"),
    TLS(true),
    Nick("discord"),
    Token("<discord bot token>"),
    Sync(Channel("#irc-channel"), Discord("#discord-channel", "<discord webhook URL>")),
    Relay(Discord("#discord-channel", "<discord webhook URL>"), Channel("#irc-read-only-channel")),
    Relay(Channel("#irc-channel"), Discord("#discord-read-only-channel", "<discord webhook URL>")),
    // ...
  )
  if err != nil {
    log.Fatal(err)
  }
  defer b.Close()

  log.Printf("IRC-Discord bridge running - press CTRL-C to exit")
  sc := make(chan os.Signal, 1)
  signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
  <-sc
}
