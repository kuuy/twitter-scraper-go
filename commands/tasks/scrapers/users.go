package scrapers

import (
  "scraper.local/twitter-scraper/commands/tasks/scrapers/users"
  "github.com/urfave/cli/v2"
)

func NewUsersCommand() *cli.Command {
  return &cli.Command{
    Name:  "users",
    Usage: "",
    Subcommands: []*cli.Command{
      users.NewPostsCommand(),
    },
  }
}
