package util

import (
	"os"
	"testing"
)

// TestIsTermux test Termux
func TestIsTermux(t *testing.T) {
	// Termux
	os.Setenv("PREFIX", "/data/data/com.termux/files/usr")

	if !isTermux() {
		t.Error("Expected isTermux to return true, got false.")
	}

	// PREFIX Termux
	os.Unsetenv("PREFIX")

	if isTermux() {
		t.Error("Expected isTermux to return false, got true.")
	}
}
