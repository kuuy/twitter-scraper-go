package commands

import (
  "scraper.local/twitter-scraper/commands/scrapers"
  "github.com/urfave/cli/v2"
)

func NewScrapersCommand() *cli.Command {
  return &cli.Command{
    Name:  "scrapers",
    Usage: "",
    Subcommands: []*cli.Command{
      scrapers.NewPostsCommand(),
      scrapers.NewRepliesCommand(),
      scrapers.NewMediaCommand(),
      scrapers.NewUsersCommand(),
    },
  }
}
