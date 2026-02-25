package util

import "os"

// isTermux Termux
//
// https://wiki.termux.com/wiki/Getting_started
func isTermux() bool {
	return os.Getenv("PREFIX") == "/data/data/com.termux/files/usr"
}
