package main

import (
	"fmt"

	"github.com/mostlygeek/reaper/token"
)

func main() {

	t, _ := token.Tokenize("test", token.NewTerminateJob("1234"))
	fmt.Println(len(t), t)
}
