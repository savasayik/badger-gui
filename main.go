package main

import (
	"context"
	"log"
	"os"

	"badge-reader/internal/app"

	"github.com/urfave/cli/v3"
)

func main() {
	appCmd := &cli.Command{
		Name:  "badger-gui",
		Usage: "View Badger DB records with Terminal",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "dbpath",
				Aliases: []string{"d"},
				Usage:   "Badger DB directory",
				Value:   "./data/badger",
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			return app.Run(c.String("dbpath"))
		},
	}

	if err := appCmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
