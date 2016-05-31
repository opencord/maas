package main

type Storage interface {
	Init(start string, count uint) error
	Get(mac string) (string, error)
	GetAll() map[string]string
	Put(mac, ip string) error
	Remove(mac string) (string, error)
	Dequeue() (string, error)
	Enqueue(ip string) error
}

type MemoryStorage struct {
	allocated               map[string]IPv4
	available               []IPv4
	readIdx, writeIdx, size uint
}

func (s *MemoryStorage) Init(start string, count uint) error {
	ip, err := ParseIP(start)
	if err != nil {
		return err
	}
	s.readIdx = 0
	s.writeIdx = 0
	s.size = count
	s.allocated = make(map[string]IPv4)
	s.available = make([]IPv4, count)
	for i := uint(0); i < count; i += 1 {
		s.available[i] = ip
		ip, err = ip.Next()
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *MemoryStorage) Get(mac string) (string, error) {
	ip, ok := s.allocated[mac]
	if !ok {
		return "", nil
	}
	return ip.String(), nil
}

func (s *MemoryStorage) GetAll() map[string]string {
	all := make(map[string]string)
	for k, v := range s.allocated {
		all[k] = v.String()
	}
	return all
}

func (s *MemoryStorage) Put(mac, ip string) error {
	data, err := ParseIP(ip)
	if err != nil {
		return err
	}
	s.allocated[mac] = data
	return nil
}

func (s *MemoryStorage) Remove(mac string) (string, error) {
	ip, ok := s.allocated[mac]
	if !ok {
		return "", nil
	}
	delete(s.allocated, mac)
	return ip.String(), nil
}

func (s *MemoryStorage) Dequeue() (string, error) {
	ip := s.available[s.readIdx]
	s.readIdx = (s.readIdx + 1) % s.size
	return ip.String(), nil
}

func (s *MemoryStorage) Enqueue(ip string) error {
	data, err := ParseIP(ip)
	if err != nil {
		return err
	}
	s.available[s.writeIdx] = data
	s.writeIdx = (s.writeIdx + 1) % s.size
	return nil
}
