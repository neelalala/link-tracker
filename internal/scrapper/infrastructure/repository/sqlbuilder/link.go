package sqlbuilder

import (
	"context"
	"errors"
	"fmt"
	"github.com/doug-martin/goqu/v9"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type LinkRepository struct {
	pool *pgxpool.Pool
}

func NewLinkRepository(pool *pgxpool.Pool) *LinkRepository {
	return &LinkRepository{
		pool: pool,
	}
}

func (linkRepo *LinkRepository) Save(ctx context.Context, link domain.Link) (domain.Link, error) {
	query, args, err := psql.Insert("links").
		Rows(goqu.Record{
			"url":          link.URL,
			"last_updated": link.LastUpdated,
		}).
		OnConflict(goqu.DoUpdate("url", goqu.Record{
			"last_updated": link.LastUpdated,
		})).
		Returning("id", "url", "last_updated").
		ToSQL()

	if err != nil {
		return domain.Link{}, fmt.Errorf("failed to build query: %w", err)
	}

	var saved domain.Link
	err = linkRepo.pool.QueryRow(ctx, query, args...).Scan(&saved.ID, &saved.URL, &saved.LastUpdated)
	if err != nil {
		return domain.Link{}, fmt.Errorf("failed to save link: %w", err)
	}

	return saved, nil
}

func (linkRepo *LinkRepository) GetById(ctx context.Context, id int64) (domain.Link, error) {
	query, args, err := psql.From("links").
		Select("id", "url", "last_updated").
		Where(goqu.Ex{"id": id}).
		ToSQL()
	if err != nil {
		return domain.Link{}, fmt.Errorf("failed to build query: %w", err)
	}

	var link domain.Link
	err = linkRepo.pool.QueryRow(ctx, query, args...).Scan(
		&link.ID,
		&link.URL,
		&link.LastUpdated,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Link{}, fmt.Errorf("%w: link with id %d not found", domain.ErrLinkNotFound, id)
		}
		return domain.Link{}, fmt.Errorf("failed to get link by id %d: %w", id, err)
	}

	return link, nil
}

func (linkRepo *LinkRepository) GetByUrl(ctx context.Context, url string) (domain.Link, error) {
	query, args, err := psql.From("links").
		Select("id", "url", "last_updated").
		Where(goqu.Ex{"url": url}).
		ToSQL()
	if err != nil {
		return domain.Link{}, fmt.Errorf("failed to build query: %w", err)
	}

	var link domain.Link
	err = linkRepo.pool.QueryRow(ctx, query, args...).Scan(
		&link.ID,
		&link.URL,
		&link.LastUpdated,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Link{}, fmt.Errorf("%w: link with url %s not found", domain.ErrLinkNotFound, url)
		}
		return domain.Link{}, fmt.Errorf("failed to get link by url: %w", err)
	}

	return link, nil
}

func (linkRepo *LinkRepository) Delete(ctx context.Context, link domain.Link) error {
	query, args, err := psql.Delete("links").
		Where(goqu.Ex{"id": link.ID}).
		ToSQL()
	if err != nil {
		return fmt.Errorf("failed to build query: %w", err)
	}

	cmdTag, err := linkRepo.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete link: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("%w: link with id %d did not exist", domain.ErrLinkNotFound, link.ID)
	}

	return nil
}

func (linkRepo *LinkRepository) GetBatch(ctx context.Context, limit int, offset int) ([]domain.Link, error) {
	query, args, err := psql.From("links").
		Select("id", "url", "last_updated").
		Order(goqu.I("id").Asc()).
		Limit(uint(limit)).
		Offset(uint(offset)).
		ToSQL()

	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	rows, err := linkRepo.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get batch of links: %w", err)
	}
	defer rows.Close()

	links := make([]domain.Link, 0, limit)

	for rows.Next() {
		var link domain.Link
		if err := rows.Scan(&link.ID, &link.URL, &link.LastUpdated); err != nil {
			return nil, fmt.Errorf("failed to scan link in batch: %w", err)
		}
		links = append(links, link)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over links batch: %w", err)
	}

	return links, nil
}
