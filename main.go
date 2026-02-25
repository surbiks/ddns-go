package main

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/jeessy2/ddns-go/v6/config"
	"github.com/jeessy2/ddns-go/v6/dns"
	"github.com/jeessy2/ddns-go/v6/util"
	"github.com/jeessy2/ddns-go/v6/util/osutil"
	"github.com/jeessy2/ddns-go/v6/util/update"
	"github.com/jeessy2/ddns-go/v6/web"
	"github.com/kardianos/service"
)

// ddns-go
// ddns-go version
var versionFlag = flag.Bool("v", false, "ddns-go version")

// update ddns-go
var updateFlag = flag.Bool("u", false, "Upgrade ddns-go to the latest version")

// listen address
var listen = flag.String("l", ":9876", "Listen address")

// update ( )
var every = flag.Int("f", 300, "Update frequency(seconds)")

// cache times
var ipCacheTimes = flag.Int("cacheTimes", 5, "Cache times")

// service management
var serviceType = flag.String("s", "", "Service management (install|uninstall|restart)")

// config file path
var configFilePath = flag.String("c", util.GetConfigFilePathDefault(), "Custom configuration file path")

// Web service
var noWebService = flag.Bool("noweb", false, "No web service")

// verify
var skipVerify = flag.Bool("skipVerify", false, "Skip certificate verification")

// DNS service
var customDNS = flag.String("dns", "", "Custom DNS server address, example: 8.8.8.8")

// reset password
var newPassword = flag.String("resetPassword", "", "Reset password to the one entered")

// run in background
var daemonize = flag.Bool("d", false, "Run in background (daemon/detached)")

//go:embed static
var staticEmbeddedFiles embed.FS

//go:embed favicon.ico
var faviconEmbeddedFile embed.FS

// version
var version = "DEV"

func main() {
	flag.Parse()
	if *versionFlag {
		fmt.Println(version)
		return
	}
	if *updateFlag {
		update.Self(version)
		return
	}

	if *daemonize && os.Getenv("DDNS_GO_DAEMON") != "1" {
		if err := runAsDaemon(); err != nil {
			log.Fatalf("Daemonize failed: %v", err)
		}
		return
	}

	// go/src/time/zoneinfo_android.go localLoc UTC
	if runtime.GOOS == "android" {
		util.FixTimezone()
	}
	// checklisten address
	if _, err := net.ResolveTCPAddr("tcp", *listen); err != nil {
		log.Fatalf("Parse listen address failed! Exception: %s", err)
	}
	// set version
	os.Setenv(web.VersionEnv, version)
	// set config file path
	if *configFilePath != "" {
		absPath, _ := filepath.Abs(*configFilePath)
		os.Setenv(util.ConfigFilePathENV, absPath)
	}
	// reset password
	if *newPassword != "" {
		conf, err := config.GetConfigCached()
		if err == nil {
			conf.ResetPassword(*newPassword)
		} else {
			util.Log("Config file %s does not exist, you can specify the configuration file through -c", *configFilePath)
		}
		return
	}
	// set skip certificate verification
	if *skipVerify {
		util.SetInsecureSkipVerify()
	}
	// set custom DNS
	if *customDNS != "" {
		util.SetDNS(*customDNS)
	}
	os.Setenv(util.IPCacheTimesENV, strconv.Itoa(*ipCacheTimes))
	switch *serviceType {
	case "install":
		installService()
	case "uninstall":
		uninstallService()
	case "restart":
		restartService()
	default:
		if util.IsRunInDocker() || os.Getenv("DDNS_GO_DAEMON") == "1" {
			run()
		} else {
			s := getService()
			status, _ := s.Status()
			if status != service.StatusUnknown {
				// service
				s.Run()
			} else {
				// service
				switch s.Platform() {
				case "windows-service":
					util.Log("You can use '.\\ddns-go.exe -s install' to install service")
				default:
					util.Log("You can use 'sudo ./ddns-go -s install' to install service")
				}
				run()
			}
		}
	}
}

func run() {
	// compatible with previous config file
	conf, _ := config.GetConfigCached()
	conf.CompatibleConfig()
	// initialize language
	util.InitLogLang(conf.Lang)

	if !*noWebService {
		go func() {
			// start web service
			err := runWebServer()
			if err != nil {
				log.Println(err)
				time.Sleep(time.Minute)
				os.Exit(1)
			}
		}()
	}

	// DNS
	util.InitBackupDNS(*customDNS, conf.Lang)

	// wait for network connection
	util.WaitInternet(dns.Addresses)

	// run periodically
	dns.RunTimer(time.Duration(*every) * time.Second)
}

func staticFsFunc(writer http.ResponseWriter, request *http.Request) {
	http.FileServer(http.FS(staticEmbeddedFiles)).ServeHTTP(writer, request)
}

func faviconFsFunc(writer http.ResponseWriter, request *http.Request) {
	http.FileServer(http.FS(faviconEmbeddedFile)).ServeHTTP(writer, request)
}

func runWebServer() error {
	// start static file service
	http.HandleFunc("/static/", web.AuthAssert(staticFsFunc))
	http.HandleFunc("/favicon.ico", web.AuthAssert(faviconFsFunc))
	http.HandleFunc("/login", web.AuthAssert(web.Login))
	http.HandleFunc("/loginFunc", web.AuthAssert(web.LoginFunc))

	http.HandleFunc("/", web.Auth(web.Writing))
	http.HandleFunc("/save", web.Auth(web.Save))
	http.HandleFunc("/logs", web.Auth(web.Logs))
	http.HandleFunc("/clearLog", web.Auth(web.ClearLog))
	http.HandleFunc("/webhookTest", web.Auth(web.WebhookTest))
	http.HandleFunc("/logout", web.Auth(web.Logout))

	util.Log("Listening on %s", *listen)

	l, err := net.Listen("tcp", *listen)
	if err != nil {
		return errors.New(util.LogStr("Port listening failed, please check if the port is occupied! %s", err))
	}

	return http.Serve(l, nil)
}

// / Unix setsid Windows DETACHED_PROCESS
func runAsDaemon() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	// -d parameters
	args := make([]string, 0, len(os.Args))
	args = append(args, exe)
	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "-d" {
			continue
		}
		args = append(args, os.Args[i])
	}

	//
	nullFile, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer nullFile.Close()

	proc, err := osutil.StartDetachedProcess(exe, args, nullFile)
	if err != nil {
		return err
	}

	return proc.Release()
}

type program struct{}

func (p *program) Start(s service.Service) error {
	// Start should not block. Do the actual work async.
	go p.run()
	return nil
}
func (p *program) run() {
	run()
}
func (p *program) Stop(s service.Service) error {
	// Stop should not block. Return with a few seconds.
	return nil
}

func getService() service.Service {
	options := make(service.KeyValue)
	var depends []string

	// service start
	switch service.ChosenSystem().String() {
	case "unix-systemv":
		options["SysvScript"] = sysvScript
	case "windows-service":
		// Windows service starttype ( start)
		options["DelayedAutoStart"] = true
	default:
		// Systemd add
		depends = append(depends, "Requires=network.target",
			"After=network-online.target")
	}

	svcConfig := &service.Config{
		Name:         "ddns-go",
		DisplayName:  "ddns-go",
		Description:  "Simple and easy to use DDNS. Automatically update domain name resolution to public IP (Support Aliyun, Tencent Cloud, Dnspod, Cloudflare, Callback, Huawei Cloud, Baidu Cloud, Porkbun, GoDaddy...)",
		Arguments:    []string{"-l", *listen, "-f", strconv.Itoa(*every), "-cacheTimes", strconv.Itoa(*ipCacheTimes), "-c", *configFilePath},
		Dependencies: depends,
		Option:       options,
	}

	if *noWebService {
		svcConfig.Arguments = append(svcConfig.Arguments, "-noweb")
	}

	if *skipVerify {
		svcConfig.Arguments = append(svcConfig.Arguments, "-skipVerify")
	}

	if *customDNS != "" {
		svcConfig.Arguments = append(svcConfig.Arguments, "-dns", *customDNS)
	}

	prg := &program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatalln(err)
	}
	return s
}

// uninstall service
func uninstallService() {
	s := getService()
	s.Stop()
	if service.ChosenSystem().String() == "unix-systemv" {
		if _, err := exec.Command("/etc/init.d/ddns-go", "stop").Output(); err != nil {
			log.Println(err)
		}
	}
	if err := s.Uninstall(); err == nil {
		util.Log("ddns-go service uninstalled successfully")
	} else {
		util.Log("ddns-go service uninstall failed, Exception: %s", err)
	}
}

// install service
func installService() {
	s := getService()

	status, err := s.Status()
	if err != nil && status == service.StatusUnknown {
		// service createservice
		if err = s.Install(); err == nil {
			s.Start()
			util.Log("Installed ddns-go service successfully! Please open the browser and configure it")
			if service.ChosenSystem().String() == "unix-systemv" {
				if _, err := exec.Command("/etc/init.d/ddns-go", "enable").Output(); err != nil {
					log.Println(err)
				}
				if _, err := exec.Command("/etc/init.d/ddns-go", "start").Output(); err != nil {
					log.Println(err)
				}
			}
			return
		}
		util.Log("Failed to install ddns-go service, Exception: %s", err)
	}

	if status != service.StatusUnknown {
		util.Log("ddns-go service has been installed, no need to install again")
	}
}

// restart service
func restartService() {
	s := getService()
	status, err := s.Status()
	if err == nil {
		if status == service.StatusRunning {
			if err = s.Restart(); err == nil {
				util.Log("restarted ddns-go service successfully")
			}
		} else if status == service.StatusStopped {
			if err = s.Start(); err == nil {
				util.Log("started ddns-go service successfully")
			}
		}
	} else {
		util.Log("ddns-go service is not installed, please install the service first")
	}
}

const sysvScript = `#!/bin/sh /etc/rc.common
DESCRIPTION="{{.Description}}"
cmd="{{.Path}}{{range .Arguments}} {{.|cmd}}{{end}}"
name="ddns-go"
pid_file="/var/run/$name.pid"
stdout_log="/var/log/$name.log"
stderr_log="/var/log/$name.err"
START=99
get_pid() {
    cat "$pid_file"
}
is_running() {
    [ -f "$pid_file" ] && cat /proc/$(get_pid)/stat > /dev/null 2>&1
}
start() {
	if is_running; then
		echo "Already started"
	else
		echo "Starting $name"
		{{if .WorkingDirectory}}cd '{{.WorkingDirectory}}'{{end}}
		$cmd >> "$stdout_log" 2>> "$stderr_log" &
		echo $! > "$pid_file"
		if ! is_running; then
			echo "Unable to start, see $stdout_log and $stderr_log"
			exit 1
		fi
	fi
}
stop() {
	if is_running; then
		echo -n "Stopping $name.."
		kill $(get_pid)
		for i in $(seq 1 10)
		do
			if ! is_running; then
				break
			fi
			echo -n "."
			sleep 1
		done
		echo
		if is_running; then
			echo "Not stopped; may still be shutting down or shutdown may have failed"
			exit 1
		else
			echo "Stopped"
			if [ -f "$pid_file" ]; then
				rm "$pid_file"
			fi
		fi
	else
		echo "Not running"
	fi
}
restart() {
	stop
	if is_running; then
		echo "Unable to stop, will not attempt to start"
		exit 1
	fi
	start
}
`
