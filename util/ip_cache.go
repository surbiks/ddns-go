package util

import (
	"os"
	"strconv"
)

const IPCacheTimesENV = "DDNS_IP_CACHE_TIMES"

// IpCache IP
type IpCache struct {
	Addr          string // address
	Times         int    //
	TimesFailedIP int    // getipfailed
}

var ForceCompareGlobal = true

func (d *IpCache) Check(newAddr string) bool {
	if newAddr == "" {
		return true
	}
	// address
	if d.Addr != newAddr || d.Times <= 1 {
		IPCacheTimes, err := strconv.Atoi(os.Getenv(IPCacheTimesENV))
		if err != nil {
			IPCacheTimes = 5
		}
		d.Addr = newAddr
		d.Times = IPCacheTimes + 1
		return true
	}
	d.Addr = newAddr
	d.Times--
	return false
}
