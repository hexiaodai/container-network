package fn

import (
	"os"
	"sync"
)

var netnsLock = sync.Mutex{}

func ActiveContailer() (map[string]struct{}, error) {
	netnsLock.Lock()
	defer netnsLock.Unlock()

	files, err := os.ReadDir("/var/run/netns")
	if err != nil {
		return nil, err
	}

	activeContailer := map[string]struct{}{}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		activeContailer[file.Name()] = struct{}{}
	}
	return activeContailer, nil
}

func Containers() ([]string, error) {
	files, err := os.ReadDir("/var/run/netns")
	if err != nil {
		return nil, err
	}
	containers := []string{}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		containers = append(containers, file.Name())
	}
	return containers, nil
}
