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
    Channel("#irc-channel", "#discord-channel", "<discord webhook URL>"),
    // Channel("#another-irc-channel", "#another-discord-channel", "<discord webhook URL>"),
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
