package commands

import (
  "scraper.local/twitter-scraper/commands/tor"
  "github.com/urfave/cli/v2"
)

func NewTorCommand() *cli.Command {
  return &cli.Command{
    Name:  "tor",
    Usage: "",
    Subcommands: []*cli.Command{
      tor.NewBridgesCommand(),
      tor.NewProxiesCommand(),
      tor.NewCronCommand(),
    },
  }
}
