package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"time"

	"github.com/jeessy2/ddns-go/v6/config"
	"github.com/jeessy2/ddns-go/v6/util"
)

//go:embed login.html
var loginEmbedFile embed.FS

// CookieName cookie name
const cookieName = "token"

// CookieInSystem only one cookie
var cookieInSystem = &http.Cookie{}

// service start time
var startTime = time.Now()

// save deadline
const saveLimit = time.Duration(30) * time.Minute

// loginfailed
const loginFailLockDuration = time.Duration(30) * time.Minute

// login check
type loginDetect struct {
	failedTimes uint32       // failed
	ticker      *time.Ticker // timer
}

var ld = &loginDetect{ticker: time.NewTicker(5 * time.Minute)}

// Login login page
func Login(writer http.ResponseWriter, request *http.Request) {
	tmpl, err := template.ParseFS(loginEmbedFile, "login.html")
	if err != nil {
		fmt.Println("Error happened..")
		fmt.Println(err)
		return
	}

	conf, _ := config.GetConfigCached()

	err = tmpl.Execute(writer, struct {
		EmptyUser bool //
	}{
		EmptyUser: conf.Username == "" && conf.Password == "",
	})
	if err != nil {
		fmt.Println("Error happened..")
		fmt.Println(err)
	}
}

// LoginFunc login func
func LoginFunc(w http.ResponseWriter, r *http.Request) {
	accept := r.Header.Get("Accept-Language")
	util.InitLogLang(accept)

	if ld.failedTimes >= 5 {
		loginUnlock()
		returnError(w, util.LogStr("Too many login failures, please try again later"))
		return
	}

	// read JSON data from request
	var data struct {
		Username string `json:"Username"`
		Password string `json:"Password"`
	}

	err := json.NewDecoder(r.Body).Decode(&data)

	if err != nil {
		returnError(w, err.Error())
		return
	}

	// username/password cannot be empty
	if data.Username == "" || data.Password == "" {
		returnError(w, util.LogStr("Username/Password is required"))
		return
	}

	conf, _ := config.GetConfigCached()

	// initialize username/password
	if conf.Username == "" && conf.Password == "" {
		if time.Since(startTime) > saveLimit {
			returnError(w, util.LogStr("Need to complete the username and password setting before %s, please restart ddns-go", startTime.Add(saveLimit).Format("2006-01-02 15:04:05")))
			return
		}

		conf.NotAllowWanAccess = true
		u, err := url.Parse(r.Header.Get("referer"))
		if err == nil && !util.IsPrivateNetwork(u.Host) {
			conf.NotAllowWanAccess = false
		}

		conf.Username = data.Username
		hashedPwd, err := conf.CheckPassword(data.Password)
		if err != nil {
			returnError(w, err.Error())
			return
		}
		conf.Password = hashedPwd
		conf.SaveConfig()
	}

	// login
	if data.Username == conf.Username && util.PasswordOK(conf.Password, data.Password) {
		ld.ticker.Stop()
		ld.failedTimes = 0

		// cookie 1
		timeoutDays := 1
		if conf.NotAllowWanAccess {
			// cookie 30
			timeoutDays = 30
		}

		// overwrite cookie
		cookieInSystem = &http.Cookie{
			Name:     cookieName,
			Value:    util.GenerateToken(data.Username), // generate token
			Path:     "/",
			Expires:  time.Now().AddDate(0, 0, timeoutDays), // set expiration time
			HttpOnly: true,
		}
		// write cookie
		http.SetCookie(w, cookieInSystem)

		util.Log("%q login succeeded", util.GetRequestIPStr(r))

		returnOK(w, util.LogStr("Login succeeded"), cookieInSystem.Value)
		return
	}

	ld.failedTimes = ld.failedTimes + 1
	util.Log("%q username or password is incorrect", util.GetRequestIPStr(r))
	returnError(w, util.LogStr("Username or password is incorrect"))
}

// loginUnlock login unlock, reset failed login attempts
func loginUnlock() {
	ld.failedTimes = ld.failedTimes + 1
	ld.ticker.Reset(loginFailLockDuration)

	go func(ticker *time.Ticker) {
		for range ticker.C {
			ld.failedTimes = 4
			ticker.Stop()
		}
	}(ld.ticker)

}
