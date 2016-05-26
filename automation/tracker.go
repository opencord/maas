package main

import (
	"encoding/json"
	"github.com/fzzy/radix/redis"
	"log"
	"net/url"
	"os"
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
	reply := t.client.Cmd("set", key, true)
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
	// Check the environment to see if we are linked to a redis DB
	if os.Getenv("AUTODB_ENV_REDIS_VERSION") != "" {
		tracker := new(RedisTracker)
		if spec := os.Getenv("AUTODB_PORT"); spec != "" {
			port, err := url.Parse(spec)
			checkError(err, "[error] unable to lookup to redis database : %s", err)
			tracker.client, err = redis.Dial(port.Scheme, port.Host)
			checkError(err, "[error] unable to connect to redis database : '%s' : %s", port, err)
			log.Println("[info] Using REDIS to track provisioning status of nodes")
			return tracker
		} else {
			log.Fatalf("[error] looks like we are configured for REDIS, but no PORT defined in environment")
		}
	}

	// Else fallback to an in memory tracker
	tracker := new(MemoryTracker)
	tracker.data = make(map[string]TrackerRecord)
	log.Println("[info] Using memory based structures to track provisioning status of nodes")
	return tracker
}
