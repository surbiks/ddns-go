// Based on https://github.com/Masterminds/semver/blob/v3.2.1/version.go

package semver

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// init() create
// create
var versionRegex *regexp.Regexp

// semVerRegex parse
const semVerRegex string = `v?([0-9]+)(\.[0-9]+)?(\.[0-9]+)?` +
	`(-([0-9A-Za-z\-]+(\.[0-9A-Za-z\-]+)*))?` +
	`(\+([0-9A-Za-z\-]+(\.[0-9A-Za-z\-]+)*))?`

// Version
type Version struct {
	major, minor, patch uint64
}

func init() {
	versionRegex = regexp.MustCompile("^" + semVerRegex + "$")
}

// NewVersion parse Version
// parse SemVer
// SemVer
func NewVersion(v string) (*Version, error) {
	m := versionRegex.FindStringSubmatch(v)
	if m == nil {
		return nil, fmt.Errorf("the %s, it's not a semantic version", v)
	}

	sv := &Version{}

	var err error
	sv.major, err = strconv.ParseUint(m[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse version: %s", err)
	}

	if m[2] != "" {
		sv.minor, err = strconv.ParseUint(strings.TrimPrefix(m[2], "."), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse version: %s", err)
		}
	} else {
		sv.minor = 0
	}

	if m[3] != "" {
		sv.patch, err = strconv.ParseUint(strings.TrimPrefix(m[3], "."), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse version: %s", err)
		}
	} else {
		sv.patch = 0
	}

	return sv, nil
}

// String Version
// v v
// v
func (v Version) String() string {
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "%d.%d.%d", v.major, v.minor, v.patch)

	return buf.String()
}

// GreaterThan test
func (v *Version) GreaterThan(o *Version) bool {
	return v.compare(o) > 0
}

// GreaterThanOrEqual test
func (v *Version) GreaterThanOrEqual(o *Version) bool {
	return v.compare(o) >= 0
}

// compare -1 0 1
//
// X.Y.Z
func (v *Version) compare(o *Version) int {
	//
	// result
	if d := compareSegment(v.major, o.major); d != 0 {
		return d
	}
	if d := compareSegment(v.minor, o.minor); d != 0 {
		return d
	}
	if d := compareSegment(v.patch, o.patch); d != 0 {
		return d
	}

	return 0
}

func compareSegment(v, o uint64) int {
	if v < o {
		return -1
	}
	if v > o {
		return 1
	}

	return 0
}
