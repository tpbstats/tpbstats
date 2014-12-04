package main

import (
	"flag"
)

func main() {
	var action string
	flag.StringVar(&action, "action", "generate", "action")
	flag.Parse()

	switch action {
		case "generate":
			generate()
		case "scrape":
			scrape()
		default:
			panic("Invalid action")
	}
}