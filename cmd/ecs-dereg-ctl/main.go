package main

import (
	"fmt"
	"os"
)

func mainInternal() int {
	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
		return 1
	}
	return 0
}

func main() {
	os.Exit(mainInternal())
}
