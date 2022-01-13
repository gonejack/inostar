package main

import (
	"log"
	"os"

	"github.com/gonejack/inostar/inostar"
)

func init() {
	log.SetOutput(os.Stdout)
}

func main() {
	cmd := inostar.Convert{
		Options: inostar.MustParseOption(),
	}
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
}
