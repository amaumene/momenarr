package domain

import "errors"

var (
	ErrNoMoviesFound   = errors.New("no movies found")
	ErrNoEpisodesFound = errors.New("no episodes found")
	ErrNoNZBFound      = errors.New("no nzb found")
	ErrMediaNotFound   = errors.New("media not found")
	ErrInvalidInput    = errors.New("invalid input")
	ErrDuplicateKey    = errors.New("duplicate key")
)
