package commands

import (
  "scraper.local/twitter-scraper/commands/users"
  "github.com/urfave/cli/v2"
)

func NewUsersCommand() *cli.Command {
  return &cli.Command{
    Name:  "users",
    Usage: "",
    Subcommands: []*cli.Command{
      users.NewCronCommand(),
    },
  }
}
