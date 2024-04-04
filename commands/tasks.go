package commands

import (
  "scraper.local/twitter-scraper/commands/tasks"
  "github.com/urfave/cli/v2"
)

func NewTasksCommand() *cli.Command {
  return &cli.Command{
    Name:  "tasks",
    Usage: "",
    Subcommands: []*cli.Command{
      tasks.NewCloudsCommand(),
      tasks.NewScrapersCommand(),
      tasks.NewPostsCommand(),
      tasks.NewRepliesCommand(),
    },
  }
}
