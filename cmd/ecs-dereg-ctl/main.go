package main

import (
	"os"
)

func mainInternal() int {
	err := app.Run(os.Args)
	if err != nil {
		return 1
	}
	return 0
}

func main() {
	os.Exit(mainInternal())
}
