package main

import (
	"encoding/json"
	"github.com/fzzy/radix/redis"
	consul "github.com/hashicorp/consul/api"
	"log"
	"net/url"
	"os"
	"strings"
)

type ProvisionState int8

const (
	Unprovisioned ProvisionState = iota
	ProvisionError
	Provisioning
	Provisioned
)

func (s *ProvisionState) String() string {
	switch *s {
	case Unprovisioned:
		return "UNPROVISIONED"
	case ProvisionError:
		return "PROVISIONERROR"
	case Provisioning:
		return "PROVISIONING"
	case Provisioned:
		return "PROVISIONED"
	default:
		return "UNKNOWN"
	}
}

// TrackerRecord state kept for each node to be provisioned
type TrackerRecord struct {
	State ProvisionState

	// Timeestamp maintains the time the node started provisioning, eventually will be used to time out
	// provisinion states
	Timestamp int64
}

// Tracker used to track if a node has been post deployed provisioned
type Tracker interface {
	Get(key string) (*TrackerRecord, error)
	Set(key string, record *TrackerRecord) error
	Clear(key string) error
}

type ConsulTracker struct {
	client *consul.Client
	kv     *consul.KV
}

func (c *ConsulTracker) Get(key string) (*TrackerRecord, error) {
	pair, _, err := c.kv.Get(key, nil)
	if err != nil {
		return nil, err
	}

	if pair == nil {
		var record TrackerRecord
		record.State = Unprovisioned
		return &record, nil
	}

	var record TrackerRecord
	err = json.Unmarshal([]byte(pair.Value), &record)
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (c *ConsulTracker) Set(key string, record *TrackerRecord) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	pair := &consul.KVPair{Key: key, Value: data}
	_, err = c.kv.Put(pair, nil)
	return err
}

func (c *ConsulTracker) Clear(key string) error {
	_, err := c.kv.Delete(key, nil)
	return err
}

// RedisTracker redis implementation of the tracker interface
type RedisTracker struct {
	client *redis.Client
}

func (t *RedisTracker) Get(key string) (*TrackerRecord, error) {
	reply := t.client.Cmd("get", key)
	if reply.Err != nil {
		return nil, reply.Err
	}
	if reply.Type == redis.NilReply {
		var record TrackerRecord
		record.State = Unprovisioned
		return &record, nil
	}

	value, err := reply.Str()
	if err != nil {
		return nil, err
	}
	var record TrackerRecord
	err = json.Unmarshal([]byte(value), &record)
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (t *RedisTracker) Set(key string, record *TrackerRecord) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	reply := t.client.Cmd("set", key, data)
	return reply.Err
}

func (t *RedisTracker) Clear(key string) error {
	reply := t.client.Cmd("del", key)
	return reply.Err
}

// MemoryTracker in memory implementation of the tracker interface
type MemoryTracker struct {
	data map[string]TrackerRecord
}

func (m *MemoryTracker) Get(key string) (*TrackerRecord, error) {
	if value, ok := m.data[key]; ok {
		return &value, nil
	}
	var record TrackerRecord
	record.State = Unprovisioned
	return &record, nil
}

func (m *MemoryTracker) Set(key string, record *TrackerRecord) error {
	m.data[key] = *record
	return nil
}

func (m *MemoryTracker) Clear(key string) error {
	delete(m.data, key)
	return nil
}

// NetTracker constructs an implemetation of the Tracker interface. Which implementation selected
//            depends on the environment. If a link to a redis instance is defined then this will
//            be used, else an in memory version will be used.
func NewTracker() Tracker {
	driver := os.Getenv("AUTODB_DRIVER")
	if driver == "" {
		log.Printf("[info] No driver specified, defaulting to in memeory persistence driver")
		driver = "MEMORY"
	}

	switch strings.ToUpper(driver) {
	case "REDIS":
		tracker := new(RedisTracker)
		if spec := os.Getenv("AUTODB_PORT"); spec != "" {
			port, err := url.Parse(spec)
			checkError(err, "[error] unable to lookup to redis database : %s", err)
			tracker.client, err = redis.Dial(port.Scheme, port.Host)
			checkError(err, "[error] unable to connect to redis database : '%s' : %s", port, err)
			log.Println("[info] Using REDIS to track provisioning status of nodes")
			return tracker
		}
		log.Fatalf("[error] No connection specified to REDIS server")
	case "CONSUL":
		var err error
		config := consul.Config{
			Address: "autodb:8500",
			Scheme:  "http",
		}
		tracker := new(ConsulTracker)
		tracker.client, err = consul.NewClient(&config)
		checkError(err, "[error] unable to connect to redis server : 'autodb:8500' : %s", err)
		log.Println("[info] Using Consul to track provisioning status of nodes")
		tracker.kv = tracker.client.KV()
		return tracker
	case "MEMORY":
		tracker := new(MemoryTracker)
		tracker.data = make(map[string]TrackerRecord)
		log.Println("[info] Using memory based structures to track provisioning status of nodes")
		return tracker
	default:
		log.Fatalf("[error] Unknown persistance driver specified, '%s'", driver)
	}
	return nil
}
