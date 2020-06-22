package bridge

import (
  "fmt"
  "log"
  "os"
  "os/signal"
  "syscall"
  "time"
)

func ExampleBot() {
  general := NewChannel("#general", "https://discord.com/api/webhooks/<webhook id>/<webhook token>", func(nick, text string) {
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

  b := NewBot("<bot token>", general)
  if err := b.Start(); err != nil {
    log.Fatal(err)
  }
  defer b.Close()

  log.Printf("Discord bridge bot running - press CTRL-C to exit")
  sc := make(chan os.Signal, 1)
  signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
  <-sc
}
