package sql

import (
	"context"
	"errors"
	"fmt"
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
	query := `
		INSERT INTO links (url, last_updated)
		VALUES ($1, $2)
		ON CONFLICT (url) DO UPDATE 
		SET last_updated = EXCLUDED.last_updated
		RETURNING id, url, last_updated
	`

	var saved domain.Link
	err := linkRepo.pool.QueryRow(ctx, query, link.URL, link.LastUpdated).Scan(
		&saved.ID,
		&saved.URL,
		&saved.LastUpdated,
	)

	if err != nil {
		return domain.Link{}, fmt.Errorf("failed to save link: %w", err)
	}

	return saved, nil
}

func (linkRepo *LinkRepository) GetById(ctx context.Context, id int64) (domain.Link, error) {
	query := `SELECT id, url, last_updated FROM links WHERE id = $1`

	var link domain.Link
	err := linkRepo.pool.QueryRow(ctx, query, id).Scan(
		&link.ID,
		&link.URL,
		&link.LastUpdated,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Link{}, fmt.Errorf("%w: link with id %d not found: %v", domain.ErrLinkNotFound, id, err.Error())
		}
		return domain.Link{}, fmt.Errorf("%w: failed to get link by id %d", err, id)
	}

	return link, nil
}

func (linkRepo *LinkRepository) GetByUrl(ctx context.Context, url string) (domain.Link, error) {
	query := `SELECT id, url, last_updated FROM links WHERE url = $1`

	var link domain.Link
	err := linkRepo.pool.QueryRow(ctx, query, url).Scan(
		&link.ID,
		&link.URL,
		&link.LastUpdated,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Link{}, fmt.Errorf("%w: link with url %s not found: %v", domain.ErrLinkNotFound, url, err.Error())
		}
		return domain.Link{}, fmt.Errorf("failed to get link by url: %w", err)
	}

	return link, nil
}

func (linkRepo *LinkRepository) Delete(ctx context.Context, link domain.Link) error {
	query := `DELETE FROM links WHERE id = $1`

	cmdTag, err := linkRepo.pool.Exec(ctx, query, link.ID)
	if err != nil {
		return fmt.Errorf("failed to delete link: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("%w: link with id %d did not exist", domain.ErrLinkNotFound, link.ID)
	}

	return nil
}

func (linkRepo *LinkRepository) GetBatch(ctx context.Context, limit int, offset int) ([]domain.Link, error) {
	query := `
		SELECT id, url, last_updated 
		FROM links 
		ORDER BY id 
		LIMIT $1 OFFSET $2
	`

	rows, err := linkRepo.pool.Query(ctx, query, limit, offset)
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
