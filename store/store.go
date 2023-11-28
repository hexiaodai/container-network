package store

import (
	"container-network/fn"
	"errors"
	"os"
	"sync"

	"gopkg.in/yaml.v2"
)

var myStore *Store

func Instance() *Store {
	if myStore == nil {
		value := fn.Args("cfgPath")
		if len(value) == 0 {
			panic("failed to parse args. arg: cfgPath")
		}
		myStore = New(value)
	}
	return myStore
}

func New(dbPath string) *Store {
	return &Store{
		Path: dbPath,
	}
}

type Store struct {
	Path string
	sync.Mutex
}

func (s *Store) Join(name, ip, cidr string) error {
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
	cluster.Node = append(cluster.Node, &Node{Name: name, IP: ip, CIDR: cidr})

	yamldata, err := yaml.Marshal(cluster)
	if err != nil {
		return err
	}

	return os.WriteFile(s.Path, yamldata, 0644)
}

func (s *Store) WriteContainer(node string, container *Container) error {
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

	for _, n := range cluster.Node {
		if n.Name == node {
			n.Containers = append(n.Containers, container)
			break
		}
	}

	yamldata, err := yaml.Marshal(cluster)
	if err != nil {
		return err
	}

	return os.WriteFile(s.Path, yamldata, 0644)
}

func (s *Store) ReadContainer(node, container string) (*Container, error) {
	s.Lock()
	defer s.Unlock()

	data, err := os.ReadFile(s.Path)
	if err != nil {
		return nil, err
	}
	cluster := &Cluster{}
	if err := yaml.Unmarshal(data, cluster); err != nil {
		return nil, err
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
	s.Lock()
	defer s.Unlock()

	data, err := os.ReadFile(s.Path)
	if err != nil {
		return nil, err
	}
	cluster := &Cluster{}
	if err := yaml.Unmarshal(data, cluster); err != nil {
		return nil, err
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
	IP         string       `yaml:"ip"`
	CIDR       string       `yaml:"cidr"`
	Gateway    string       `yaml:"gateway"`
	Containers []*Container `yaml:"containers"`
}

type Container struct {
	Status        string `yaml:"status"`
	Name          string `yaml:"name"`
	IP            string `yaml:"ip"`
	Veth0         string `yaml:"veth0"`
	Veth1         string `yaml:"veth1"`
	ContainerPort int    `yaml:"containerPort"`
	HostPort      int    `yaml:"hostPort"`
}
