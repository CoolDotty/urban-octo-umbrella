package main

import (
	"database/sql"
	"embed"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"

	_ "urban-octo-umbrella/server/migrations"
)

//go:embed web/dist/*
var embeddedFiles embed.FS

const authCookieName = "pb_auth"

type signupPayload struct {
	Email           string `json:"email"`
	Password        string `json:"password"`
	PasswordConfirm string `json:"passwordConfirm"`
	InviteToken     string `json:"inviteToken"`
}

type loginPayload struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func main() {
	app := pocketbase.New()

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		e.Router.BindFunc(func(re *core.RequestEvent) error {
			if re.Request.Header.Get("Authorization") == "" {
				if cookie, err := re.Request.Cookie(authCookieName); err == nil && cookie.Value != "" {
					re.Request.Header.Set("Authorization", "Bearer "+cookie.Value)
				}
			}

			return re.Next()
		})

		fsys, err := fs.Sub(embeddedFiles, "web/dist")
		if err != nil {
			return err
		}

		indexHTML, err := fs.ReadFile(fsys, "index.html")
		if err != nil {
			return err
		}

		e.Router.GET("/auth/signup-config", func(re *core.RequestEvent) error {
			total, err := app.CountRecords("users")
			if err != nil {
				return re.JSON(http.StatusInternalServerError, map[string]string{
					"message": "Failed to read signup configuration.",
				})
			}

			return re.JSON(http.StatusOK, map[string]any{
				"requiresInvite": total > 0,
				"userCount":      total,
			})
		})

		e.Router.POST("/auth/signup", func(re *core.RequestEvent) error {
			var payload signupPayload
			if err := re.BindBody(&payload); err != nil {
				return re.JSON(http.StatusBadRequest, map[string]string{
					"message": "Invalid signup payload.",
				})
			}

			payload.Email = strings.TrimSpace(payload.Email)
			if payload.Email == "" || payload.Password == "" || payload.PasswordConfirm == "" {
				return re.JSON(http.StatusBadRequest, map[string]string{
					"message": "Email and password are required.",
				})
			}

			if payload.Password != payload.PasswordConfirm {
				return re.JSON(http.StatusBadRequest, map[string]string{
					"message": "Passwords do not match.",
				})
			}

			var createdUser *core.Record
			var authToken string

			err := app.RunInTransaction(func(txApp core.App) error {
				total, countErr := txApp.CountRecords("users")
				if countErr != nil {
					return countErr
				}

				var invite *core.Record
				if total > 0 {
					if payload.InviteToken == "" {
						return errors.New("invite_required")
					}

					found, findErr := txApp.FindFirstRecordByFilter(
						"invites",
						"token = {:token}",
						dbx.Params{"token": payload.InviteToken},
					)
					if findErr != nil {
						return findErr
					}

					usedAt := found.GetDateTime("used_at")
					if !usedAt.IsZero() {
						return errors.New("invite_used")
					}

					expiresAt := found.GetDateTime("expires_at")
					if !expiresAt.IsZero() && expiresAt.Before(types.NowDateTime()) {
						return errors.New("invite_expired")
					}

					invite = found
				}

				existing, findErr := txApp.FindAuthRecordByEmail("users", payload.Email)
				if findErr == nil && existing != nil {
					return errors.New("email_taken")
				}
				if findErr != nil && !errors.Is(findErr, sql.ErrNoRows) {
					return findErr
				}

				usersCollection, colErr := txApp.FindCollectionByNameOrId("users")
				if colErr != nil {
					return colErr
				}

				user := core.NewRecord(usersCollection)
				user.SetEmail(payload.Email)
				user.SetPassword(payload.Password)
				if total == 0 {
					user.Set("role", "admin")
				} else {
					user.Set("role", "user")
				}

				if saveErr := txApp.Save(user); saveErr != nil {
					return saveErr
				}

				if invite != nil {
					invite.Set("used_at", types.NowDateTime())
					invite.Set("used_by", user.Id)
					if saveErr := txApp.Save(invite); saveErr != nil {
						return saveErr
					}
				}

				token, tokenErr := user.NewAuthToken()
				if tokenErr != nil {
					return tokenErr
				}

				createdUser = user
				authToken = token

				return nil
			})

			if err != nil {
				switch {
				case errors.Is(err, sql.ErrNoRows):
					return re.JSON(http.StatusForbidden, map[string]string{
						"message": "Invalid invite token.",
					})
				case err.Error() == "invite_required":
					return re.JSON(http.StatusForbidden, map[string]string{
						"message": "Invite token required.",
					})
				case err.Error() == "invite_used":
					return re.JSON(http.StatusForbidden, map[string]string{
						"message": "Invite token already used.",
					})
				case err.Error() == "invite_expired":
					return re.JSON(http.StatusForbidden, map[string]string{
						"message": "Invite token expired.",
					})
				case err.Error() == "email_taken":
					return re.JSON(http.StatusConflict, map[string]string{
						"message": "Email already registered.",
					})
				default:
					return re.JSON(http.StatusInternalServerError, map[string]string{
						"message": "Failed to create account.",
					})
				}
			}

			setAuthCookie(re, authToken)

			return re.JSON(http.StatusCreated, publicUser(createdUser))
		})

		e.Router.POST("/auth/login", func(re *core.RequestEvent) error {
			var payload loginPayload
			if err := re.BindBody(&payload); err != nil {
				return re.JSON(http.StatusBadRequest, map[string]string{
					"message": "Invalid login payload.",
				})
			}

			payload.Email = strings.TrimSpace(payload.Email)
			if payload.Email == "" || payload.Password == "" {
				return re.JSON(http.StatusBadRequest, map[string]string{
					"message": "Email and password are required.",
				})
			}

			record, err := app.FindAuthRecordByEmail("users", payload.Email)
			if err != nil || record == nil || !record.ValidatePassword(payload.Password) {
				return re.JSON(http.StatusUnauthorized, map[string]string{
					"message": "Invalid email or password.",
				})
			}

			token, tokenErr := record.NewAuthToken()
			if tokenErr != nil {
				return re.JSON(http.StatusInternalServerError, map[string]string{
					"message": "Failed to create session.",
				})
			}

			setAuthCookie(re, token)

			return re.JSON(http.StatusOK, publicUser(record))
		})

		e.Router.POST("/auth/logout", func(re *core.RequestEvent) error {
			clearAuthCookie(re)
			return re.NoContent(http.StatusNoContent)
		})

		e.Router.GET("/auth/me", func(re *core.RequestEvent) error {
			if re.Auth == nil {
				return re.JSON(http.StatusUnauthorized, map[string]string{
					"message": "Unauthorized.",
				})
			}

			return re.JSON(http.StatusOK, publicUser(re.Auth))
		})

		e.Router.GET("/*", func(re *core.RequestEvent) error {
			path := strings.TrimPrefix(re.Request.URL.Path, "/")
			if path == "" {
				path = "index.html"
			}

			if _, err := fs.Stat(fsys, path); err == nil {
				return re.FileFS(fsys, path)
			}

			return re.HTML(http.StatusOK, string(indexHTML))
		})

		return e.Next()
	})

	if err := app.Start(); err != nil {
		panic(err)
	}
}

func publicUser(record *core.Record) map[string]any {
	if record == nil {
		return map[string]any{}
	}

	return map[string]any{
		"id":           record.Id,
		"email":        record.Email(),
		"role":         record.GetString("role"),
		"display_name": record.GetString("display_name"),
	}
}

func setAuthCookie(re *core.RequestEvent, token string) {
	cookie := &http.Cookie{
		Name:     authCookieName,
		Value:    token,
		HttpOnly: true,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
		Secure:   resolveCookieSecure(re.Request),
	}

	domain := strings.TrimSpace(os.Getenv("AUTH_COOKIE_DOMAIN"))
	if domain != "" {
		cookie.Domain = domain
	}

	re.SetCookie(cookie)
}

func clearAuthCookie(re *core.RequestEvent) {
	cookie := &http.Cookie{
		Name:     authCookieName,
		Value:    "",
		HttpOnly: true,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
		Secure:   resolveCookieSecure(re.Request),
		MaxAge:   -1,
	}

	domain := strings.TrimSpace(os.Getenv("AUTH_COOKIE_DOMAIN"))
	if domain != "" {
		cookie.Domain = domain
	}

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
