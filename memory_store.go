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
	nextVisitID  int64
	links        map[int64]link
	linkVisits   map[int64]linkVisit
	shortNameIDs map[string]int64
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		nextID:       1,
		nextVisitID:  1,
		links:        make(map[int64]link),
		linkVisits:   make(map[int64]linkVisit),
		shortNameIDs: make(map[string]int64),
	}
}

func (s *memoryStore) ListLinks(_ context.Context, params listLinksParams) ([]link, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	links := make([]link, 0, len(s.links))
	for _, item := range s.links {
		links = append(links, item)
	}

	sort.Slice(links, func(i, j int) bool {
		return links[i].ID < links[j].ID
	})

	start := min(int(params.Offset), len(links))
	end := min(start+int(params.Limit), len(links))

	return links[start:end], nil
}

func (s *memoryStore) CountLinks(_ context.Context) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return int64(len(s.links)), nil
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
	for visitID, visit := range s.linkVisits {
		if visit.LinkID == id {
			delete(s.linkVisits, visitID)
		}
	}

	return nil
}

func (s *memoryStore) ListLinkVisits(_ context.Context, params listLinkVisitsParams) ([]linkVisit, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	visits := make([]linkVisit, 0, len(s.linkVisits))
	for _, item := range s.linkVisits {
		visits = append(visits, item)
	}

	sort.Slice(visits, func(i, j int) bool {
		return visits[i].ID < visits[j].ID
	})

	start := min(int(params.Offset), len(visits))
	end := min(start+int(params.Limit), len(visits))

	return visits[start:end], nil
}

func (s *memoryStore) CountLinkVisits(_ context.Context) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return int64(len(s.linkVisits)), nil
}

func (s *memoryStore) CreateLinkVisit(_ context.Context, params createLinkVisitParams) (linkVisit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.links[params.LinkID]; !ok {
		return linkVisit{}, errLinkNotFound
	}

	item := linkVisit{
		ID:        s.nextVisitID,
		LinkID:    params.LinkID,
		CreatedAt: time.Now().UTC(),
		IP:        params.IP,
		UserAgent: params.UserAgent,
		Referer:   params.Referer,
		Status:    params.Status,
	}

	s.linkVisits[item.ID] = item
	s.nextVisitID++

	return item, nil
}
