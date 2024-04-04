package commands

import (
  "scraper.local/twitter-scraper/commands/queue"
  "github.com/urfave/cli/v2"
)

func NewQueueCommand() *cli.Command {
  return &cli.Command{
    Name:  "queue",
    Usage: "",
    Subcommands: []*cli.Command{
      queue.NewAsynqCommand(),
      queue.NewNatsCommand(),
    },
  }
}
