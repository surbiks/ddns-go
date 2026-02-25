package web

import (
	"net/http"
	"time"
)

func Logout(w http.ResponseWriter, r *http.Request) {
	// overwrite cookieInSystem
	cookieInSystem = &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0), // set as expired time
		MaxAge:   -1,              // delete Cookie
		HttpOnly: true,
	}
	// set expired cookie
	http.SetCookie(w, cookieInSystem)

	// redirect user to login page
	http.Redirect(w, r, "./login", http.StatusFound)
}
