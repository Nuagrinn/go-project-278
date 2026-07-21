package main

import (
	"context"
	"sort"
	"sync"
	"time"
)

type memoryStore struct {
	mu           sync.RWMutex
	nextID       int64
	links        map[int64]link
	shortNameIDs map[string]int64
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		nextID:       1,
		links:        make(map[int64]link),
		shortNameIDs: make(map[string]int64),
	}
}

func (s *memoryStore) ListLinks(_ context.Context) ([]link, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	links := make([]link, 0, len(s.links))
	for _, item := range s.links {
		links = append(links, item)
	}

	sort.Slice(links, func(i, j int) bool {
		return links[i].ID < links[j].ID
	})

	return links, nil
}

func (s *memoryStore) GetLink(_ context.Context, id int64) (link, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	item, ok := s.links[id]
	if !ok {
		return link{}, errLinkNotFound
	}

	return item, nil
}

func (s *memoryStore) GetLinkByShortName(_ context.Context, shortName string) (link, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, ok := s.shortNameIDs[shortName]
	if !ok {
		return link{}, errLinkNotFound
	}

	return s.links[id], nil
}

func (s *memoryStore) CreateLink(_ context.Context, params createLinkParams) (link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.shortNameIDs[params.ShortName]; ok {
		return link{}, errDuplicateShortName
	}

	item := link{
		ID:          s.nextID,
		OriginalURL: params.OriginalURL,
		ShortName:   params.ShortName,
		CreatedAt:   time.Now().UTC(),
	}

	s.links[item.ID] = item
	s.shortNameIDs[item.ShortName] = item.ID
	s.nextID++

	return item, nil
}

func (s *memoryStore) UpdateLink(_ context.Context, params updateLinkParams) (link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.links[params.ID]
	if !ok {
		return link{}, errLinkNotFound
	}

	if existingID, ok := s.shortNameIDs[params.ShortName]; ok && existingID != params.ID {
		return link{}, errDuplicateShortName
	}

	delete(s.shortNameIDs, item.ShortName)

	item.OriginalURL = params.OriginalURL
	item.ShortName = params.ShortName
	s.links[item.ID] = item
	s.shortNameIDs[item.ShortName] = item.ID

	return item, nil
}

func (s *memoryStore) DeleteLink(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.links[id]
	if !ok {
		return errLinkNotFound
	}

	delete(s.links, id)
	delete(s.shortNameIDs, item.ShortName)

	return nil
}
