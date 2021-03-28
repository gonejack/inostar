package main

import (
	"log"
	"os"

	"github.com/gonejack/inoreader-starred/cmd"
	"github.com/spf13/cobra"
)

var (
	verbose = false
	prog    = &cobra.Command{
		Use:   "inoreader-starred *.json",
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
		Verbose:   verbose,
	}
	return exec.Execute(args)
}

func main() {
	_ = prog.Execute()
}
