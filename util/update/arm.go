// Based on https://github.com/creativeprojects/go-selfupdate/blob/v1.1.1/arm.go

package update

import (
	// unsafe runtime get
	_ "unsafe"
)

//go:linkname goarm runtime.goarm
var goarm uint8
