package main

import (
	"fmt"
	"github.com/virajsazzala/swrm/internal/bencode"
)

func main() {
	// test
	inp := "5:hello"
	bencode.Unmarshal([]byte(inp))
	fmt.Println([]byte("23"))
}