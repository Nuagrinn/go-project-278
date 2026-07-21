package main

import (
	"context"
	"database/sql"
	"errors"

	"code/internal/db"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type postgresStore struct {
	db      *sql.DB
	queries *db.Queries
}

func newPostgresStore(database *sql.DB) *postgresStore {
	return &postgresStore{
		db:      database,
		queries: db.New(database),
	}
}

func (s *postgresStore) ListLinks(ctx context.Context, params listLinksParams) ([]link, error) {
	rows, err := s.queries.ListLinks(ctx, db.ListLinksParams{
		Limit:  params.Limit,
		Offset: params.Offset,
	})
	if err != nil {
		return nil, err
	}

	links := make([]link, 0, len(rows))
	for _, row := range rows {
		links = append(links, toLink(row))
	}

	return links, nil
}

func (s *postgresStore) CountLinks(ctx context.Context) (int64, error) {
	return s.queries.CountLinks(ctx)
}

func (s *postgresStore) GetLink(ctx context.Context, id int64) (link, error) {
	row, err := s.queries.GetLink(ctx, id)
	if err != nil {
		return link{}, mapStoreError(err)
	}

	return toLink(row), nil
}

func (s *postgresStore) GetLinkByShortName(ctx context.Context, shortName string) (link, error) {
	row, err := s.queries.GetLinkByShortName(ctx, shortName)
	if err != nil {
		return link{}, mapStoreError(err)
	}

	return toLink(row), nil
}

func (s *postgresStore) CreateLink(ctx context.Context, params createLinkParams) (link, error) {
	row, err := s.queries.CreateLink(ctx, db.CreateLinkParams{
		OriginalUrl: params.OriginalURL,
		ShortName:   params.ShortName,
	})
	if err != nil {
		return link{}, mapStoreError(err)
	}

	return toLink(row), nil
}

func (s *postgresStore) UpdateLink(ctx context.Context, params updateLinkParams) (link, error) {
	row, err := s.queries.UpdateLink(ctx, db.UpdateLinkParams{
		ID:          params.ID,
		OriginalUrl: params.OriginalURL,
		ShortName:   params.ShortName,
	})
	if err != nil {
		return link{}, mapStoreError(err)
	}

	return toLink(row), nil
}

func (s *postgresStore) DeleteLink(ctx context.Context, id int64) error {
	rowsAffected, err := s.queries.DeleteLink(ctx, id)
	if err != nil {
		return mapStoreError(err)
	}

	if rowsAffected == 0 {
		return errLinkNotFound
	}

	return nil
}

func (s *postgresStore) Close() error {
	return s.db.Close()
}

func toLink(row db.Link) link {
	return link{
		ID:          row.ID,
		OriginalURL: row.OriginalUrl,
		ShortName:   row.ShortName,
		CreatedAt:   row.CreatedAt,
	}
}

func mapStoreError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return errLinkNotFound
	}

	if isUniqueViolation(err) {
		return errDuplicateShortName
	}

	return err
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError

	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
