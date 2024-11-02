package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println("Code is running")
    for {
        // Keeping the main function active
        time.Sleep(time.Hour)
    }}