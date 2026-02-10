package main

import "errors"

const (
	CollectionUsers   = "users"
	CollectionInvites = "invites"

	RoleAdmin = "admin"
	RoleUser  = "user"
)

var (
	errInviteRequired = errors.New("invite required")
	errInviteUsed     = errors.New("invite already used")
	errInviteExpired  = errors.New("invite expired")
	errEmailTaken     = errors.New("email already registered")
)

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
