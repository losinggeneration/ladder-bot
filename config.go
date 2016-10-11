package main

import "os"

var accessToken = ""

func init() {
	t := os.Getenv("ACCESS_TOKEN")
	if t != "" {
		accessToken = t
	}
}
