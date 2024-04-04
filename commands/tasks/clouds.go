package tasks

import (
  "scraper.local/twitter-scraper/commands/tasks/clouds"
  "github.com/urfave/cli/v2"
)

func NewCloudsCommand() *cli.Command {
  return &cli.Command{
    Name:  "clouds",
    Usage: "",
    Subcommands: []*cli.Command{
      clouds.NewMediaCommand(),
    },
  }
}
