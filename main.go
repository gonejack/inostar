package main

import (
	"log"
	"os"

	"github.com/gonejack/inostar/cmd"
)

func init() {
	log.SetOutput(os.Stdout)
}

func main() {
	c := cmd.Convert{
		ImagesDir: "images",
	}
	if err := c.Run(); err != nil {
		log.Fatal(err)
	}
}
