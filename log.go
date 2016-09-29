package main

import "log"

var debug bool

func setDebug(enable bool) {
	debug = enable
}

// Debug will conditionally log a debug message
func Debug(args ...interface{}) {
	if debug {
		log.Print(append([]interface{}{"DEBUG: "}, args...)...)
	}
}

// Debugf will conditionally log a formatted debug message
func Debugf(format string, args ...interface{}) {
	if debug {
		log.Printf("DEBUG: "+format, args...)
	}
}
