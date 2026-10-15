package main

import (
	"fmt"
	"time"
)

func main() {
	now := time.Now().UTC().Add(3 * time.Hour)
	fmt.Printf("Moscow time: %s\n", now.Format("2006-01-02 15:04:05"))
}