// Based on https://github.com/creativeprojects/go-selfupdate/blob/v1.1.1/detect.go

package update

import (
	"fmt"
	"log"
	"runtime"
	"strings"

	"github.com/jeessy2/ddns-go/v6/util/semver"
)

// detectLatest get
func detectLatest(repo string) (latest *Latest, found bool, err error) {
	rel, err := getLatest(repo)
	if err != nil {
		return nil, false, err
	}

	asset, ver, found := findAsset(rel)
	if !found {
		return nil, false, nil
	}

	return newLatest(asset, ver), true, nil
}

// findAsset asset
func findAsset(rel *Release) (*Asset, *semver.Version, bool) {
	// ARM
	//
	for _, arch := range append(generateAdditionalArch(), runtime.GOARCH) {
		asset, version, found := findAssetForArch(arch, rel)
		if found {
			return asset, version, found
		}
	}

	return nil, nil, false
}

func findAssetForArch(arch string, rel *Release,
) (asset *Asset, version *semver.Version, found bool) {
	var release *Release

	// release
	// GitHub API create
	if a, v, ok := findAssetFromRelease(rel, getSuffixes(arch)); ok {
		version = v
		asset = a
		release = rel
	}

	if release == nil {
		log.Printf("Cannot find any release for %s/%s", runtime.GOOS, runtime.GOARCH)
		return nil, nil, false
	}

	return asset, version, true
}

func findAssetFromRelease(rel *Release, suffixes []string) (*Asset, *semver.Version, bool) {
	if rel == nil {
		log.Print("There is no source release information")
		return nil, nil, false
	}

	// parse
	ver, err := semver.NewVersion(rel.tagName)
	if err != nil {
		log.Printf("Cannot parse semantic version: %s", rel.tagName)
		return nil, nil, false
	}

	for _, asset := range rel.assets {
		if assetMatchSuffixes(asset.name, suffixes) {
			return &asset, ver, true
		}
	}

	log.Printf("Can't find suitable asset in release %s", rel.tagName)
	return nil, nil, false
}

func assetMatchSuffixes(name string, suffixes []string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(name, suffix) { //
			//
			return true
		}
	}
	return false
}

// getSuffixes asset check
//
// TODO: get MIPS float get MIPS
func getSuffixes(arch string) []string {
	suffixes := make([]string, 0)
	for _, ext := range []string{".zip", ".tar.gz"} {
		suffix := fmt.Sprintf("%s_%s%s", runtime.GOOS, arch, ext)
		suffixes = append(suffixes, suffix)
	}
	return suffixes
}
