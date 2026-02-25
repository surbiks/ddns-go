package web

import (
	"net/http"
	"time"

	"github.com/jeessy2/ddns-go/v6/config"
	"github.com/jeessy2/ddns-go/v6/util"
)

// ViewFunc func
type ViewFunc func(http.ResponseWriter, *http.Request)

// Auth verifyToken
func Auth(f ViewFunc) ViewFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookieInWeb, err := r.Cookie(cookieName)
		if err != nil {
			http.Redirect(w, r, "./login", http.StatusTemporaryRedirect)
			return
		}

		conf, _ := config.GetConfigCached()

		// deny public network access
		if conf.NotAllowWanAccess {
			if !util.IsPrivateNetwork(r.RemoteAddr) {
				w.WriteHeader(http.StatusForbidden)
				util.Log("%q is prohibited from accessing the public network", util.GetRequestIPStr(r))
				return
			}
		}

		// verifytoken
		if cookieInSystem.Value != "" &&
			cookieInSystem.Value == cookieInWeb.Value &&
			cookieInSystem.Expires.After(time.Now()) {
			f(w, r) // execute wrapped function
			return
		}

		http.Redirect(w, r, "./login", http.StatusTemporaryRedirect)
	}
}

// AuthAssert file
func AuthAssert(f ViewFunc) ViewFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		conf, err := config.GetConfigCached()

		// config file is empty, start 3
		if err != nil &&
			time.Since(startTime) > time.Duration(3*time.Hour) && !util.IsPrivateNetwork(r.RemoteAddr) {
			w.WriteHeader(http.StatusForbidden)
			util.Log("%q configuration file is empty, public network access is prohibited for more than 3 hours", util.GetRequestIPStr(r))
			return
		}

		// deny public network access
		if conf.NotAllowWanAccess {
			if !util.IsPrivateNetwork(r.RemoteAddr) {
				w.WriteHeader(http.StatusForbidden)
				util.Log("%q is prohibited from accessing the public network", util.GetRequestIPStr(r))
				return
			}
		}

		f(w, r) // execute wrapped function

	}
}
