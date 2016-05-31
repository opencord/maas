package main

func Allocate(store Storage, mac string) (string, error) {
	// Check to see if an IP address is already allocated and if so just
	// return that value
	ip, err := store.Get(mac)
	if err != nil {
		return "", err
	}

	if ip != "" {
		return ip, nil
	}

	// This MAC does not already have an IP assigned, so pull then next
	// one off the available queue and return it
	ip, err = store.Dequeue()
	if err != nil {
		return "", err
	}
	err = store.Put(mac, ip)
	if err != nil {
		store.Enqueue(ip)
		return "", err
	}
	return ip, nil
}

func Release(store Storage, mac string) error {
	ip, err := store.Remove(mac)
	if err != nil {
		return err
	}

	if ip != "" {
		err = store.Enqueue(ip)
		if err != nil {
			return err
		}
	}
	return nil
}
