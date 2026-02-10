package main

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
	"github.com/pocketbase/pocketbase/tools/types"
)

func registerAuthRoutes(router *router.Router[*core.RequestEvent], app *pocketbase.PocketBase) {
	router.GET("/auth/signup-config", func(re *core.RequestEvent) error {
		total, err := app.CountRecords(CollectionUsers)
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

	router.POST("/auth/signup", func(re *core.RequestEvent) error {
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
			total, countErr := txApp.CountRecords(CollectionUsers)
			if countErr != nil {
				return countErr
			}

			var invite *core.Record
			if total > 0 {
				if payload.InviteToken == "" {
					return errInviteRequired
				}

				found, findErr := txApp.FindFirstRecordByFilter(
					CollectionInvites,
					"token = {:token}",
					dbx.Params{"token": payload.InviteToken},
				)
				if findErr != nil {
					return findErr
				}

				usedAt := found.GetDateTime("used_at")
				if !usedAt.IsZero() {
					return errInviteUsed
				}

				expiresAt := found.GetDateTime("expires_at")
				if !expiresAt.IsZero() && expiresAt.Before(types.NowDateTime()) {
					return errInviteExpired
				}

				invite = found
			}

			existing, findErr := txApp.FindAuthRecordByEmail(CollectionUsers, payload.Email)
			if findErr == nil && existing != nil {
				return errEmailTaken
			}
			if findErr != nil && !errors.Is(findErr, sql.ErrNoRows) {
				return findErr
			}

			usersCollection, colErr := txApp.FindCollectionByNameOrId(CollectionUsers)
			if colErr != nil {
				return colErr
			}

			user := core.NewRecord(usersCollection)
			user.SetEmail(payload.Email)
			user.SetPassword(payload.Password)
			if total == 0 {
				user.Set("role", RoleAdmin)
			} else {
				user.Set("role", RoleUser)
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
			case errors.Is(err, errInviteRequired):
				return re.JSON(http.StatusForbidden, map[string]string{
					"message": "Invite token required.",
				})
			case errors.Is(err, errInviteUsed):
				return re.JSON(http.StatusForbidden, map[string]string{
					"message": "Invite token already used.",
				})
			case errors.Is(err, errInviteExpired):
				return re.JSON(http.StatusForbidden, map[string]string{
					"message": "Invite token expired.",
				})
			case errors.Is(err, errEmailTaken):
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

	router.POST("/auth/login", func(re *core.RequestEvent) error {
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

		record, err := app.FindAuthRecordByEmail(CollectionUsers, payload.Email)
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

	router.POST("/auth/logout", func(re *core.RequestEvent) error {
		clearAuthCookie(re)
		return re.NoContent(http.StatusNoContent)
	})

	router.GET("/auth/me", func(re *core.RequestEvent) error {
		if re.Auth == nil {
			return re.JSON(http.StatusUnauthorized, map[string]string{
				"message": "Unauthorized.",
			})
		}

		return re.JSON(http.StatusOK, publicUser(re.Auth))
	})
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
