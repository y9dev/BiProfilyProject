// Package users — sentinel-ошибки репозитория.
package users

import "errors"

var (
	ErrLoginTaken = errors.New("login_taken")
	errBadRole    = errors.New("bad role")
	errBadStatus  = errors.New("bad status")
)
