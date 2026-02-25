package update

import (
	"fmt"
	"io"
	"log"
	"os"
	"runtime"

	"github.com/jeessy2/ddns-go/v6/util"
	"github.com/jeessy2/ddns-go/v6/util/semver"
)

// Self update ddns-go
func Self(version string) {
	//
	v, err := semver.NewVersion(version)
	if err != nil {
		log.Printf("Cannot update because: %v", err)
		return
	}

	latest, found, err := detectLatest("jeessy2/ddns-go")
	if err != nil {
		log.Printf("Error happened when detecting latest version: %v", err)
		return
	}
	if !found {
		log.Printf("Cannot find any release for %s/%s", runtime.GOOS, runtime.GOARCH)
		return
	}
	if v.GreaterThanOrEqual(latest.Version) {
		log.Printf("Current version (%s) is the latest", version)
		return
	}

	exe, err := os.Executable()
	if err != nil {
		log.Printf("Cannot find executable path: %v", err)
		return
	}

	if err = to(latest.URL, latest.Name, exe); err != nil {
		log.Printf("Error happened when updating binary: %v", err)
		return
	}

	log.Printf("Success update to v%s", latest.Version.String())
}

// to assetURL file file file
// update file API HTTP URL asset
// update release
// cmdPath file filepath
func to(assetURL, assetFileName, cmdPath string) error {
	src, err := downloadAssetFromURL(assetURL)
	if err != nil {
		return err
	}
	defer src.Close()
	return decompressAndUpdate(src, assetFileName, cmdPath)
}

func downloadAssetFromURL(url string) (rc io.ReadCloser, err error) {
	client := util.CreateHTTPClient()
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("could not download release from %s: %v", url, err)
	}
	if resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, fmt.Errorf("could not download release from %s. Response code: %d", url, resp.StatusCode)
	}

	return resp.Body, nil
}
