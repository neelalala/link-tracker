package link

import (
	"context"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"sort"
	"sync"
)

type MemoryRepository struct {
	mu     sync.RWMutex
	links  map[string]domain.Link
	nextID int64
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		links:  make(map[string]domain.Link),
		nextID: 1,
	}
}

func (linkRepo *MemoryRepository) Save(ctx context.Context, link domain.Link) (domain.Link, error) {
	linkRepo.mu.Lock()
	defer linkRepo.mu.Unlock()
	if link.ID == 0 {
		link.ID = linkRepo.nextID
		linkRepo.nextID++
	}
	linkRepo.links[link.URL] = link
	return link, nil
}

func (linkRepo *MemoryRepository) GetById(ctx context.Context, id int64) (domain.Link, error) {
	linkRepo.mu.RLock()
	defer linkRepo.mu.RUnlock()
	for _, link := range linkRepo.links {
		if link.ID == id {
			return link, nil
		}
	}
	return domain.Link{}, domain.ErrLinkNotFound
}

func (linkRepo *MemoryRepository) GetByUrl(ctx context.Context, url string) (domain.Link, error) {
	linkRepo.mu.RLock()
	defer linkRepo.mu.RUnlock()
	if link, ok := linkRepo.links[url]; ok {
		return link, nil
	}
	return domain.Link{}, domain.ErrLinkNotFound
}

func (linkRepo *MemoryRepository) Delete(ctx context.Context, link domain.Link) error {
	linkRepo.mu.Lock()
	defer linkRepo.mu.Unlock()

	if _, ok := linkRepo.links[link.URL]; !ok {
		return domain.ErrLinkNotFound
	}

	delete(linkRepo.links, link.URL)
	return nil
}

func (linkRepo *MemoryRepository) GetBatch(ctx context.Context, limit int, offset int) ([]domain.Link, error) {
	linkRepo.mu.RLock()
	defer linkRepo.mu.RUnlock()

	allLinks := make([]domain.Link, 0, len(linkRepo.links))
	for _, link := range linkRepo.links {
		allLinks = append(allLinks, link)
	}

	sort.Slice(allLinks, func(i, j int) bool {
		return allLinks[i].ID < allLinks[j].ID
	})

	if offset >= len(allLinks) {
		return []domain.Link{}, nil
	}

	end := offset + limit
	if end > len(allLinks) {
		end = len(allLinks)
	}

	return allLinks[offset:end], nil
}
