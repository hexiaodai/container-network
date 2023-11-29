package store

import (
	"container-network/fn"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v2"
)

var Instance *Store = new()

type Events interface {
	Update(context.Context, *Cluster)
}

func new() *Store {
	cfgPath := fn.Args("cfgPath")
	if cfgPath == "" {
		cfgPath = "config.yaml"
	}
	absPath, err := filepath.Abs(cfgPath)
	if err != nil {
		panic(err)
	}
	s := &Store{
		Path:  absPath,
		value: atomic.Value{},
	}
	if err := s.updateHandler(); err != nil {
		panic(fmt.Errorf("failed to updating store: %s", err))
	}
	return s
}

type Store struct {
	Path   string
	value  atomic.Value // *Cluster
	events []Events
	sync.Mutex
}

func (s *Store) Running(ctx context.Context, wg *sync.WaitGroup) {
	fmt.Println("running store")

	for _, e := range s.events {
		e.Update(ctx, s.value.Load().(*Cluster))
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}
	defer watcher.Close()

	if err := watcher.Add(s.Path); err != nil {
		panic(err)
	}

	fmt.Printf("Watching changes for file: %s\n", s.Path)

	for {
		select {
		case <-ctx.Done():
			wg.Done()
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Rename == fsnotify.Rename {
				if err := s.updateHandler(); err != nil {
					fn.Errorf("failed to update store: %v", err)
				}
				cluster := s.value.Load().(*Cluster)
				if cluster == nil {
					fn.Errorf("failed to load cluster from store")
					continue
				}
				fmt.Println("sending events...")
				for _, e := range s.events {
					e.Update(ctx, cluster)
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			fn.Errorf(err.Error())
		}
	}
}

func (s *Store) RegisterEvents(e Events) {
	s.events = append(s.events, e)
}

func (s *Store) updateHandler() error {
	s.Lock()
	defer s.Unlock()

	data, err := os.ReadFile(s.Path)
	if err != nil {
		return err
	}
	cluster := &Cluster{}
	if err := yaml.Unmarshal(data, cluster); err != nil {
		return err
	}
	s.value.Store(cluster)
	return nil
}

func (s *Store) ReadContainer(node, container string) (*Container, error) {
	value := s.value.Load()
	if value == nil {
		return nil, errors.New("ReadContainer: value.Load() is nil")
	}
	cluster := value.(*Cluster)
	if cluster == nil {
		return nil, fmt.Errorf("store.Cluster is nil")
	}

	for _, n := range cluster.Node {
		if n.Name == node {
			for _, c := range n.Containers {
				if c.Name == container {
					return c, nil
				}
			}
		}
	}
	return nil, errors.New("NotFound")
}

func (s *Store) ReadNode(node string) (*Node, error) {
	value := s.value.Load()
	if value == nil {
		return nil, errors.New("ReadNode: value.Load() is nil")
	}
	cluster := value.(*Cluster)
	if cluster == nil {
		return nil, fmt.Errorf("store.Cluster is nil")
	}

	for _, n := range cluster.Node {
		if n.Name == node {
			return n, nil
		}
	}
	return nil, errors.New("NotFound")
}

type Cluster struct {
	Network string  `yaml:"network"`
	Node    []*Node `yaml:"node"`
}

type Node struct {
	Name       string       `yaml:"name"`
	Inertface  string       `yaml:"interface"`
	IP         string       `yaml:"ip"`
	CIDR       string       `yaml:"cidr"`
	Gateway    string       `yaml:"gateway"`
	Containers []*Container `yaml:"containers"`
}

type Container struct {
	Name          string `yaml:"name"`
	IP            string `yaml:"ip"`
	Veth0         string `yaml:"veth0"`
	Veth1         string `yaml:"veth1"`
	ContainerPort int    `yaml:"containerPort"`
	HostPort      int    `yaml:"hostPort"`
}
