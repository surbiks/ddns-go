package util

import (
	"strings"
	"time"
)

// Wait blocks until the Internet is connected.
//
// See also:
//
//   - https://stackoverflow.com/a/50058255
//   - https://github.com/ddev/ddev/blob/v1.22.7/pkg/globalconfig/global_config.go#L776
func WaitInternet(addresses []string) {
	delay := time.Second * 5
	retryTimes := 0
	failed := false

	for {
		for _, addr := range addresses {

			err := LookupHost(addr)
			// Internet is connected.
			if err == nil {
				if failed {
					Log("The network is connected")
				}
				return
			}

			failed = true
			Log("Waiting for network connection: %s", err)
			Log("Retry after %s", delay)

			if isDNSErr(err) || retryTimes > 0 {
				dns := BackupDNS[retryTimes%len(BackupDNS)]
				Log("Local DNS exception! Will use %s by default, you can use -dns to customize DNS server", dns)
				SetDNS(dns)
				retryTimes = retryTimes + 1
			}

			time.Sleep(delay)
		}
	}
}

// isDNSErr checks if the error is caused by DNS.
func isDNSErr(e error) bool {
	return strings.Contains(e.Error(), "[::1]:53: read: connection refused")
}
