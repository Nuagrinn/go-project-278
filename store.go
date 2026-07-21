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

type listLinksParams struct {
	Offset int32
	Limit  int32
}

type linkStore interface {
	ListLinks(ctx context.Context, params listLinksParams) ([]link, error)
	CountLinks(ctx context.Context) (int64, error)
	GetLink(ctx context.Context, id int64) (link, error)
	GetLinkByShortName(ctx context.Context, shortName string) (link, error)
	CreateLink(ctx context.Context, params createLinkParams) (link, error)
	UpdateLink(ctx context.Context, params updateLinkParams) (link, error)
	DeleteLink(ctx context.Context, id int64) error
}
