package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	method := os.Getenv("REQUEST_METHOD")
	path := os.Getenv("PATH_INFO")
	_ = method
	_ = path

	utcNow := time.Now().UTC()
	now := utcNow.Add(3 * time.Hour)
	fmt.Printf(`{"unix":%d,"iso":%q,"hour_minute":%q}`+"\n",
		utcNow.Unix(),
		now.Format("2006-01-02T15:04:05+03:00"),
		now.Format("15:04"),
	)
}