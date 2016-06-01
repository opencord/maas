package main

import (
	"log"
)

type Storage interface {
	Put(id string, update StatusMsg) error
	Get(id string) (*StatusMsg, error)
	List() ([]StatusMsg, error)
}

type MemoryStorage struct {
	Data map[string]StatusMsg
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		Data: make(map[string]StatusMsg),
	}
}

func (s *MemoryStorage) Put(id string, update StatusMsg) error {
	s.Data[id] = update
	log.Printf("%s : %s", id, update.Status.String())
	return nil
}

func (s *MemoryStorage) Get(id string) (*StatusMsg, error) {
	m, ok := s.Data[id]
	if !ok {
		return nil, nil
	}
	return &m, nil
}

func (s *MemoryStorage) List() ([]StatusMsg, error) {
	r := make([]StatusMsg, len(s.Data))
	i := 0
	for _, v := range s.Data {
		r[i] = v
	}
	return r, nil
}
