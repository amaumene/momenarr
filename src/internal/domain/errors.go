package domain

import "errors"

var (
	ErrNoMoviesFound   = errors.New("no movies found")
	ErrNoEpisodesFound = errors.New("no episodes found")
	ErrNoNZBFound      = errors.New("no nzb found")
	ErrDuplicateKey    = errors.New("duplicate key")
)
