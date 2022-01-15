package inostar

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
)

type about bool

func (a about) BeforeApply() (err error) {
	fmt.Println("Visit https://github.com/gonejack/inostar")
	os.Exit(0)
	return
}

type Options struct {
	Offline bool  `short:"e" help:"Download remote images and replace html references."`
	Verbose bool  `short:"v" help:"Verbose printing."`
	About   about `help:"Show About."`

	ImagesDir string `hidden:"" default:"images"`

	JSON []string `name:"starred.json" arg:"" optional:""`
}

func MustParseOption() (opt Options) {
	kong.Parse(&opt,
		kong.Name("inostar"),
		kong.Description("This command line converts inoreader's exported starred.json into .html files"),
		kong.UsageOnError(),
	)
	return
}
