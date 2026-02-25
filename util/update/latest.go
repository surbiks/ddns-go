// Based on https://github.com/creativeprojects/go-selfupdate/blob/v1.1.1/release.go

package update

import "github.com/jeessy2/ddns-go/v6/util/semver"

// Latest release asset
type Latest struct {
	//Name asset file
	Name string
	// URL release file URL
	URL string
	// version parse *Version
	Version *semver.Version
}

func newLatest(asset *Asset, ver *semver.Version) *Latest {
	latest := &Latest{
		Name:    asset.name,
		URL:     asset.url,
		Version: ver,
	}

	return latest
}
