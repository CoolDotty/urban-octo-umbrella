package main

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

const authCookieName = "pb_auth"

func baseCookie(re *core.RequestEvent) *http.Cookie {
	cookie := &http.Cookie{
		Name:     authCookieName,
		HttpOnly: true,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
		Secure:   resolveCookieSecure(re.Request),
	}

	if domain := strings.TrimSpace(os.Getenv("AUTH_COOKIE_DOMAIN")); domain != "" {
		cookie.Domain = domain
	}

	return cookie
}

func setAuthCookie(re *core.RequestEvent, token string) {
	cookie := baseCookie(re)
	cookie.Value = token

	if maxAge := resolveCookieMaxAge(); maxAge > 0 {
		cookie.MaxAge = maxAge
		cookie.Expires = time.Now().Add(time.Duration(maxAge) * time.Second)
	}

	re.SetCookie(cookie)
}

func clearAuthCookie(re *core.RequestEvent) {
	cookie := baseCookie(re)
	cookie.MaxAge = -1

	re.SetCookie(cookie)
}

func resolveCookieSecure(req *http.Request) bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv("AUTH_COOKIE_SECURE")))
	switch raw {
	case "true":
		return true
	case "false":
		return false
	default:
		return req.TLS != nil
	}
}

func resolveCookieMaxAge() int {
	raw := strings.TrimSpace(os.Getenv("AUTH_COOKIE_TTL_DAYS"))
	if raw == "" {
		return 30 * 24 * 60 * 60
	}

	days, err := strconv.Atoi(raw)
	if err != nil || days <= 0 {
		return 0
	}

	return days * 24 * 60 * 60
}
