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

type linkVisit struct {
	ID        int64
	LinkID    int64
	CreatedAt time.Time
	IP        string
	UserAgent string
	Referer   string
	Status    int32
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

type createLinkVisitParams struct {
	LinkID    int64
	IP        string
	UserAgent string
	Referer   string
	Status    int32
}

type listLinkVisitsParams struct {
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
	ListLinkVisits(ctx context.Context, params listLinkVisitsParams) ([]linkVisit, error)
	CountLinkVisits(ctx context.Context) (int64, error)
	CreateLinkVisit(ctx context.Context, params createLinkVisitParams) (linkVisit, error)
}
