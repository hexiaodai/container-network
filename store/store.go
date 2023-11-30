package store

import (
	"bytes"
	"container-network/fn"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/julienschmidt/httprouter"
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
		nodeName: fn.Args("node"),
		path:     absPath,
		value:    atomic.Value{},
	}
	if err := s.update(); err != nil {
		panic(fmt.Errorf("failed to updating store: %s", err))
	}
	return s
}

type Store struct {
	path     string
	nodeName string
	value    atomic.Value // *Cluster
	events   []Events
	sync.Mutex
}

func (s *Store) Running(ctx context.Context, wg *sync.WaitGroup) {
	fmt.Println("running store")

	for _, e := range s.events {
		e.Update(ctx, s.value.Load().(*Cluster))
	}

	switch s.nodeName {
	case "master":
		go s.apiserver(ctx)
		s.master(ctx, wg)
	default:
		s.slave(ctx, wg)
	}
}

func (s *Store) apiserver(ctx context.Context) {
	router := httprouter.New()
	router.GET("/store", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		cluster := s.value.Load().(*Cluster)
		if cluster == nil {
			http.Error(w, "store.Cluster is nil", http.StatusBadRequest)
			return
		}
		bys, err := json.Marshal(cluster)
		if err != nil {
			http.Error(w, "failed to marshal cluster", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, string(bys))
	})
	router.POST("/vxlan/mac", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read request body", http.StatusBadRequest)
			return
		}
		var data map[string]interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			http.Error(w, "failed to parse request body", http.StatusBadRequest)
			return
		}
		nodeName := data["nodeName"].(string)
		mac := data["mac"].(string)
		if err := s.WriteVXLANMAC(nodeName, mac); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	fmt.Printf("Listening %v", fn.Args("apiserver"))
	log.Fatal(http.ListenAndServe(fn.Args("apiserver"), router))
}

func (s *Store) master(ctx context.Context, wg *sync.WaitGroup) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}
	defer watcher.Close()

	if err := watcher.Add(s.path); err != nil {
		panic(err)
	}

	fmt.Printf("Watching changes for file: %s\n", s.path)

	for {
		select {
		case <-ctx.Done():
			wg.Done()
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			//  || event.Op&fsnotify.Rename == fsnotify.Rename
			if event.Op&fsnotify.Write == fsnotify.Write {
				if err := s.updateWithFile(); err != nil {
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

func (s *Store) slave(ctx context.Context, wg *sync.WaitGroup) {
	for {
		select {
		case <-ctx.Done():
			wg.Done()
			return
		case <-time.After(time.Second * 5):
			if err := s.update(); err != nil {
				fn.Errorf("failed to update store: %v", err)
			}
		}
	}
}

func (s *Store) RegisterEvents(e Events) {
	s.events = append(s.events, e)
}

func (s *Store) update() error {
	switch s.nodeName {
	case "master":
		return s.updateWithFile()
	default:
		return s.updateWithRESTAPI()
	}
}

func (s *Store) updateWithFile() error {
	s.Lock()
	defer s.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	cluster := &Cluster{}
	if err := yaml.Unmarshal(data, cluster); err != nil {
		return err
	}
	s.value.Store(cluster)
	return nil
}

func (s *Store) updateWithRESTAPI() error {
	s.Lock()
	defer s.Unlock()

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	api := fmt.Sprintf("http://%v/store", fn.Args("apiserver"))
	req, err := http.NewRequest("GET", api, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	bysBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get store: msg: %v. statusCode: %v", string(bysBody), resp.StatusCode)
	}
	cluster := &Cluster{}
	if err := json.Unmarshal(bysBody, cluster); err != nil {
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

func (s *Store) WriteVXLANMAC(nodeName, mac string) error {
	switch s.nodeName {
	case "master":
		return s.writeVXLANMACWithFile(nodeName, mac)
	default:
		return s.writeVXLANMACWithRESTAPI(nodeName, mac)
	}
}

func (s *Store) writeVXLANMACWithFile(nodeName, mac string) error {
	s.Lock()
	defer s.Unlock()

	value := s.value.Load()
	if value == nil {
		return errors.New("WriteVXLANMAC: value.Load() is nil")
	}

	cluster := value.(*Cluster)
	if cluster == nil {
		return errors.New("WriteVXLANMAC: value.(*Cluster) is nil")
	}
	for _, node := range cluster.Node {
		if node.Name == nodeName {
			node.VXLAN.MAC = mac
			return nil
		}
	}
	return nil
}

func (s *Store) writeVXLANMACWithRESTAPI(nodeName, mac string) error {
	s.Lock()
	defer s.Unlock()

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	api := fmt.Sprintf("http://%v/vxlan/mac", fn.Args("apiserver"))
	data := map[string]string{
		"nodeName": nodeName,
		"mac":      mac,
	}
	var body []byte
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", api, bytes.NewReader(body))
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	bysBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to write vxlan mac: msg: %v. statusCode: %v", string(bysBody), resp.StatusCode)
	}
	return nil
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
	Node []*Node `yaml:"node"`
}

type Node struct {
	Name       string       `yaml:"name"`
	Inertface  string       `yaml:"interface"`
	IP         string       `yaml:"ip"`
	CIDR       string       `yaml:"cidr"`
	Gateway    string       `yaml:"gateway"`
	VXLAN      *VXLAN       `yaml:"vxlan"`
	Containers []*Container `yaml:"containers"`
}

type VXLAN struct {
	MAC string `yaml:"mac"`
	IP  string `yaml:"ip"`
}

type Container struct {
	Name          string `yaml:"name"`
	IP            string `yaml:"ip"`
	Veth0         string `yaml:"veth0"`
	Veth1         string `yaml:"veth1"`
	ContainerPort int    `yaml:"containerPort"`
	HostPort      int    `yaml:"hostPort"`
}
