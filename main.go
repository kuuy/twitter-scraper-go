package main

import (
  "log"
  "os"
  "path"
  "path/filepath"

  "scraper.local/twitter-scraper/commands"
  "github.com/joho/godotenv"
  "github.com/urfave/cli/v2"
)

func main() {
  if err := godotenv.Load(path.Join(filepath.Dir(os.Args[0]), ".env")); err != nil {
    dir, _ := os.Getwd()
    if err = godotenv.Load(path.Join(dir, ".env")); err != nil {
      panic(err)
    }
  }

  app := &cli.App{
    Name:  "twitter scraper commands",
    Usage: "",
    Action: func(c *cli.Context) error {
      if c.Command.Action == nil {
        cli.ShowAppHelp(c)
      } else {
        log.Fatalln("error", c.Err)
      }
      return nil
    },
    Commands: []*cli.Command{
      commands.NewDbCommand(),
      commands.NewSessionsCommand(),
      commands.NewTokenCommand(),
      commands.NewCloudsCommand(),
      commands.NewScrapersCommand(),
      commands.NewMediaCommand(),
      commands.NewTasksCommand(),
      commands.NewApiCommand(),
      commands.NewQueueCommand(),
      commands.NewCronCommand(),
      commands.NewUsersCommand(),
      commands.NewTorCommand(),
      commands.NewAdminsCommand(),
      commands.NewPlatformsCommand(),
    },
    Version: "0.0.0",
  }

  err := app.Run(os.Args)
  if err != nil {
    log.Fatalln("error", err)
  }
}
