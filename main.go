package main

import (
	"log"
	"os"

	"github.com/gonejack/inostar/cmd"
	"github.com/spf13/cobra"
)

var (
	offline = false
	verbose = false
	prog    = &cobra.Command{
		Use:   "inostar *.json",
		Short: "Command line tool for converting inoreader starred.json to html",
		Run: func(c *cobra.Command, args []string) {
			err := run(c, args)
			if err != nil {
				log.Fatal(err)
			}
		},
	}
)

func init() {
	log.SetOutput(os.Stdout)

	prog.Flags().SortFlags = false
	prog.PersistentFlags().SortFlags = false
	prog.PersistentFlags().BoolVarP(
		&offline,
		"offline",
		"e",
		false,
		"download remote images and replace html <img> references",
	)
	prog.PersistentFlags().BoolVarP(
		&verbose,
		"verbose",
		"v",
		false,
		"verbose",
	)
}

func run(c *cobra.Command, args []string) error {
	exec := cmd.ConvertStarred{
		ImagesDir: "images",
		Offline:   offline,
		Verbose:   verbose,
	}
	return exec.Execute(args)
}

func main() {
	_ = prog.Execute()
}
