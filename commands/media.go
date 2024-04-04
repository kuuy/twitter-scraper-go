package commands

import (
  "scraper.local/twitter-scraper/commands/media"
  "github.com/urfave/cli/v2"
)

func NewMediaCommand() *cli.Command {
  return &cli.Command{
    Name:  "media",
    Usage: "",
    Subcommands: []*cli.Command{
      media.NewPhotosCommand(),
      media.NewVideosCommand(),
    },
  }
}
