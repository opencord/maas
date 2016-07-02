package main

import (
	"encoding/json"
	consul "github.com/hashicorp/consul/api"
	"net/url"
)

const (
	PREFIX = "cord/provisioner/"
)

type ConsulStorage struct {
	client *consul.Client
	kv     *consul.KV
}

func NewConsulStorage(spec string) (*ConsulStorage, error) {
	conn, err := url.Parse(spec)
	if err != nil {
		return nil, err
	}

	cfg := consul.Config{
		Address: conn.Host,
		Scheme:  "http",
	}

	log.Debugf("Consul config = %+v", cfg)

	client, err := consul.NewClient(&cfg)
	if err != nil {
		return nil, err
	}
	return &ConsulStorage{
		client: client,
		kv:     client.KV(),
	}, nil
}

func (s *ConsulStorage) Put(id string, update StatusMsg) error {
	data, err := json.Marshal(update)
	if err != nil {
		return err
	}
	_, err = s.kv.Put(&consul.KVPair{
		Key:   PREFIX + id,
		Value: data,
	}, nil)
	return err
}

func (s *ConsulStorage) Delete(id string) error {
	_, err := s.kv.Delete(PREFIX+id, nil)
	return err
}

func (s *ConsulStorage) Get(id string) (*StatusMsg, error) {
	pair, _, err := s.kv.Get(PREFIX+id, nil)
	if err != nil {
		return nil, err
	}

	if pair == nil {
		return nil, nil
	}

	var record StatusMsg
	err = json.Unmarshal([]byte(pair.Value), &record)
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *ConsulStorage) List() ([]StatusMsg, error) {
	pairs, _, err := s.kv.List(PREFIX, nil)
	if err != nil {
		return nil, err
	}
	result := make([]StatusMsg, len(pairs))
	i := 0
	for _, pair := range pairs {
		err = json.Unmarshal([]byte(pair.Value), &(result[i]))
		if err != nil {
			return nil, err
		}
		i += 1
	}
	return result, nil
}
