package config

import (
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/jeessy2/ddns-go/v6/util"
	passwordvalidator "github.com/wagslane/go-password-validator"
	"gopkg.in/yaml.v3"
)

// Ipv4Reg IPv4
var Ipv4Reg = regexp.MustCompile(`((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])`)

// Ipv6Reg IPv6
var Ipv6Reg = regexp.MustCompile(`((([0-9A-Fa-f]{1,4}:){7}([0-9A-Fa-f]{1,4}|:))|(([0-9A-Fa-f]{1,4}:){6}(:[0-9A-Fa-f]{1,4}|((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3})|:))|(([0-9A-Fa-f]{1,4}:){5}(((:[0-9A-Fa-f]{1,4}){1,2})|:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3})|:))|(([0-9A-Fa-f]{1,4}:){4}(((:[0-9A-Fa-f]{1,4}){1,3})|((:[0-9A-Fa-f]{1,4})?:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){3}(((:[0-9A-Fa-f]{1,4}){1,4})|((:[0-9A-Fa-f]{1,4}){0,2}:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){2}(((:[0-9A-Fa-f]{1,4}){1,5})|((:[0-9A-Fa-f]{1,4}){0,3}:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:))|(([0-9A-Fa-f]{1,4}:){1}(((:[0-9A-Fa-f]{1,4}){1,6})|((:[0-9A-Fa-f]{1,4}){0,4}:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:))|(:(((:[0-9A-Fa-f]{1,4}){1,7})|((:[0-9A-Fa-f]{1,4}){0,5}:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:)))`)

// DnsConfig config
type DnsConfig struct {
	Name string
	Ipv4 struct {
		Enable bool
		// getIPtype url/netInterface
		GetType      string
		URL          string
		NetInterface string
		Cmd          string
		Domains      []string
	}
	Ipv6 struct {
		Enable bool
		// getIPtype url/netInterface
		GetType      string
		URL          string
		NetInterface string
		Cmd          string
		Ipv6Reg      string // ipv6
		Domains      []string
	}
	DNS DNS
	TTL string
}

// DNS DNSconfig
type DNS struct {
	// alidns,webhook
	Name   string
	ID     string
	Secret string
	// ExtParam parameters DNS Vercel teamId
	ExtParam string
}

type Config struct {
	DnsConf []DnsConfig
	User
	Webhook
	// deny public network access
	NotAllowWanAccess bool
	//
	Lang string
}

// ConfigCache ConfigCache
type cacheType struct {
	ConfigSingle *Config
	Err          error
	Lock         sync.Mutex
}

var cache = &cacheType{}

// GetConfigCached get config
func GetConfigCached() (conf Config, err error) {
	cache.Lock.Lock()
	defer cache.Lock.Unlock()

	if cache.ConfigSingle != nil {
		return *cache.ConfigSingle, cache.Err
	}

	// init config
	cache.ConfigSingle = &Config{}

	configFilePath := util.GetConfigFilePath()
	_, err = os.Stat(configFilePath)
	if err != nil {
		cache.Err = err
		return *cache.ConfigSingle, err
	}

	byt, err := os.ReadFile(configFilePath)
	if err != nil {
		util.Log("Exception: %s", err)
		cache.Err = err
		return *cache.ConfigSingle, err
	}

	err = yaml.Unmarshal(byt, cache.ConfigSingle)
	if err != nil {
		util.Log("Exception: %s", err)
		cache.Err = err
		return *cache.ConfigSingle, err
	}

	// login ,
	if cache.ConfigSingle.Username == "" && cache.ConfigSingle.Password == "" {
		cache.ConfigSingle.NotAllowWanAccess = true
	}

	// remove err
	cache.Err = nil
	return *cache.ConfigSingle, err
}

// CompatibleConfig compatible with previous config file
func (conf *Config) CompatibleConfig() {

	// bcrypt , save
	if conf.Password != "" && !util.IsHashedPassword(conf.Password) {
		hashedPwd, err := util.HashPassword(conf.Password)
		if err == nil {
			conf.Password = hashedPwd
			conf.SaveConfig()
		}
	}

	// compatiblev5.0.0 configfile
	if len(conf.DnsConf) > 0 {
		return
	}

	configFilePath := util.GetConfigFilePath()
	_, err := os.Stat(configFilePath)
	if err != nil {
		return
	}
	byt, err := os.ReadFile(configFilePath)
	if err != nil {
		return
	}

	dnsConf := &DnsConfig{}
	err = yaml.Unmarshal(byt, dnsConf)
	if err != nil {
		return
	}
	if len(dnsConf.DNS.Name) > 0 {
		cache.Lock.Lock()
		defer cache.Lock.Unlock()
		conf.DnsConf = append(conf.DnsConf, *dnsConf)
		cache.ConfigSingle = conf
	}
}

// SaveConfig saveconfig
func (conf *Config) SaveConfig() (err error) {
	cache.Lock.Lock()
	defer cache.Lock.Unlock()

	byt, err := yaml.Marshal(conf)
	if err != nil {
		log.Println(err)
		return err
	}

	configFilePath := util.GetConfigFilePath()
	err = os.WriteFile(configFilePath, byt, 0600)
	if err != nil {
		log.Println(err)
		return
	}

	util.Log("Config file has been saved to: %s", configFilePath)

	// config
	cache.ConfigSingle = nil

	return
}

// reset password
func (conf *Config) ResetPassword(newPassword string) {
	// initialize language
	util.InitLogLang(conf.Lang)

	// check
	hashedPwd, err := conf.CheckPassword(newPassword)
	if err != nil {
		util.Log(err.Error())
		return
	}

	// saveconfig
	conf.Password = hashedPwd
	conf.SaveConfig()
	util.Log("Password for username %s has been reset successfully! Please restart ddns-go", conf.Username)
}

// CheckPassword check
func (conf *Config) CheckPassword(newPassword string) (hashedPwd string, err error) {
	var minEntropyBits float64 = 30
	if conf.NotAllowWanAccess {
		minEntropyBits = 25
	}
	err = passwordvalidator.Validate(newPassword, minEntropyBits)
	if err != nil {
		return "", errors.New(util.LogStr("Password is not secure! Try using a more complex password"))
	}

	//
	hashedPwd, err = util.HashPassword(newPassword)
	if err != nil {
		return "", errors.New(util.LogStr("Exception: %s", err.Error()))
	}
	return
}

func (conf *DnsConfig) getIpv4AddrFromInterface() string {
	ipv4, _, err := GetNetInterface()
	if err != nil {
		util.Log("Failed to get IPv4 from network card")
		return ""
	}

	for _, netInterface := range ipv4 {
		if netInterface.Name == conf.Ipv4.NetInterface && len(netInterface.Address) > 0 {
			return netInterface.Address[0]
		}
	}

	util.Log("Failed to get IPv4 from network card! Network card name: %s", conf.Ipv4.NetInterface)
	return ""
}

func (conf *DnsConfig) getIpv4AddrFromUrl() string {
	client := util.CreateNoProxyHTTPClient("tcp4")
	urls := strings.Split(conf.Ipv4.URL, ",")
	for _, url := range urls {
		url = strings.TrimSpace(url)
		resp, err := client.Get(url)
		if err != nil {
			util.Log("Failed to get IPv4 from %s", url)
			util.Log("Exception: %s", err)
			continue
		}
		defer resp.Body.Close()
		lr := io.LimitReader(resp.Body, 1024000)
		body, err := io.ReadAll(lr)
		if err != nil {
			util.Log("Exception: %s", err)
			continue
		}
		result := Ipv4Reg.FindString(string(body))
		if result == "" {
			util.Log("Failed to get IPv4 result! Interface: %s ,Result: %s", url, string(body))
		}
		return result
	}
	return ""
}

func (conf *DnsConfig) getAddrFromCmd(addrType string) string {
	var cmd string
	var comp *regexp.Regexp
	if addrType == "IPv4" {
		cmd = conf.Ipv4.Cmd
		comp = Ipv4Reg
	} else {
		cmd = conf.Ipv6.Cmd
		comp = Ipv6Reg
	}
	// cmd is empty
	if cmd == "" {
		return ""
	}
	// run cmd with proper shell
	var execCmd *exec.Cmd
	if runtime.GOOS == "windows" {
		execCmd = exec.Command("powershell", "-Command", cmd)
	} else {
		// If Bash does not exist, use sh
		_, err := exec.LookPath("bash")
		if err != nil {
			execCmd = exec.Command("sh", "-c", cmd)
		} else {
			execCmd = exec.Command("bash", "-c", cmd)
		}
	}
	// run cmd
	out, err := execCmd.CombinedOutput()
	if err != nil {
		util.Log("Failed to get %s result! Command: %s, Error: %q, Exit status code: %s", addrType, execCmd.String(), out, err)
		return ""
	}
	str := string(out)
	// get result
	result := comp.FindString(str)
	if result == "" {
		util.Log("Failed to get %s result! Command: %s, Stdout: %q", addrType, execCmd.String(), str)
	}
	return result
}

// GetIpv4Addr getIPv4address
func (conf *DnsConfig) GetIpv4Addr() string {
	// getIP
	switch conf.Ipv4.GetType {
	case "netInterface":
		// get IP
		return conf.getIpv4AddrFromInterface()
	case "url":
		// URL get IP
		return conf.getIpv4AddrFromUrl()
	case "cmd":
		// get IP
		return conf.getAddrFromCmd("IPv4")
	default:
		log.Println("IPv4's get IP method is unknown")
		return "" // unknown type
	}
}

func (conf *DnsConfig) getIpv6AddrFromInterface() string {
	_, ipv6, err := GetNetInterface()
	if err != nil {
		util.Log("Failed to get IPv6 from network card")
		return ""
	}

	for _, netInterface := range ipv6 {
		if netInterface.Name == conf.Ipv6.NetInterface && len(netInterface.Address) > 0 {
			if conf.Ipv6.Ipv6Reg != "" {
				// IPv6
				if match, err := regexp.MatchString("@\\d", conf.Ipv6.Ipv6Reg); err == nil && match {
					num, err := strconv.Atoi(conf.Ipv6.Ipv6Reg[1:])
					if err == nil {
						if num > 0 {
							if num <= len(netInterface.Address) {
								return netInterface.Address[num-1]
							}
							util.Log("%dth IPv6 address not found! Will use the first IPv6 address", num)
							return netInterface.Address[0]
						}
						util.Log("IPv6 match expression %s is incorrect! Minimum start from 1", conf.Ipv6.Ipv6Reg)
						return ""
					}
				}
				//
				util.Log("IPv6 will use regular expression %s for matching", conf.Ipv6.Ipv6Reg)
				for i := 0; i < len(netInterface.Address); i++ {
					matched, err := regexp.MatchString(conf.Ipv6.Ipv6Reg, netInterface.Address[i])
					if matched && err == nil {
						util.Log("Match successfully! Matched address: %s", netInterface.Address[i])
						return netInterface.Address[i]
					}
				}
				util.Log("No IPv6 address matched, will use the first address")
			}
			return netInterface.Address[0]
		}
	}

	util.Log("Failed to get IPv6 from network card! Network card name: %s", conf.Ipv6.NetInterface)
	return ""
}

func (conf *DnsConfig) getIpv6AddrFromUrl() string {
	client := util.CreateNoProxyHTTPClient("tcp6")
	urls := strings.Split(conf.Ipv6.URL, ",")
	for _, url := range urls {
		url = strings.TrimSpace(url)
		resp, err := client.Get(url)
		if err != nil {
			util.Log("Failed to get IPv6 from %s", url)
			util.Log("Exception: %s", err)
			continue
		}

		defer resp.Body.Close()
		lr := io.LimitReader(resp.Body, 1024000)
		body, err := io.ReadAll(lr)
		if err != nil {
			util.Log("Exception: %s", err)
			continue
		}
		result := Ipv6Reg.FindString(string(body))
		if result == "" {
			util.Log("Failed to get IPv6 result! Interface: %s ,Result: %s", url, result)
		}
		return result
	}
	return ""
}

// GetIpv6Addr getIPv6address
func (conf *DnsConfig) GetIpv6Addr() (result string) {
	// getIP
	switch conf.Ipv6.GetType {
	case "netInterface":
		// get IP
		return conf.getIpv6AddrFromInterface()
	case "url":
		// URL get IP
		return conf.getIpv6AddrFromUrl()
	case "cmd":
		// get IP
		return conf.getAddrFromCmd("IPv6")
	default:
		log.Println("IPv6's get IP method is unknown")
		return "" // unknown type
	}
}
