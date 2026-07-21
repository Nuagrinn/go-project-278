package main

import (
	"context"
	"errors"
	"time"
)

var (
	errDuplicateShortName = errors.New("short name already exists")
	errLinkNotFound       = errors.New("link not found")
)

type link struct {
	ID          int64
	OriginalURL string
	ShortName   string
	CreatedAt   time.Time
}

type createLinkParams struct {
	OriginalURL string
	ShortName   string
}

type updateLinkParams struct {
	ID          int64
	OriginalURL string
	ShortName   string
}

type linkStore interface {
	ListLinks(ctx context.Context) ([]link, error)
	GetLink(ctx context.Context, id int64) (link, error)
	GetLinkByShortName(ctx context.Context, shortName string) (link, error)
	CreateLink(ctx context.Context, params createLinkParams) (link, error)
	UpdateLink(ctx context.Context, params updateLinkParams) (link, error)
	DeleteLink(ctx context.Context, id int64) error
}
