package cluster

import (
	"container-network/containerd"
	"container-network/fn"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/julienschmidt/httprouter"
	"gopkg.in/yaml.v2"
)

var Instance *Cluster = New()

func New() *Cluster {
	return &Cluster{}
}

type Cluster struct {
	Current *Node   `yaml:"current"`
	Nodes   []*Node `yaml:"nodes"`
}

func (c *Cluster) init() error {
	cfgPath := fn.Args("cfgPath")
	if len(cfgPath) == 0 {
		cfgPath = "config.yaml"
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(data, c); err != nil {
		return err
	}
	return nil
}

func (c *Cluster) Running(ctx context.Context) {
	if err := c.init(); err != nil {
		panic(err)
	}

	router := httprouter.New()
	router.GET("/vxlan/mac", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		if len(c.Current.VXLAN.MAC) == 0 {
			http.Error(w, "not ready", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, string(c.Current.VXLAN.MAC))
	})
	router.GET("/containers", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		containers := []*containerd.Container{}
		for _, container := range containerd.Instance.List() {
			containers = append(containers, container)
		}
		bys, err := json.Marshal(containers)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, string(bys))
	})

	log.Fatal(http.ListenAndServe(fmt.Sprintf("%v:8080", c.Current.IP), router))
}

func (c *Cluster) GetVXLANMAC(ctx context.Context, nodeIP string) (string, error) {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	api := fmt.Sprintf("http://%v/vxlan/mac", fmt.Sprintf("%v:8080", nodeIP))
	req, err := http.NewRequest("GET", api, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	bysBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get vxlan mac: msg: %v. statusCode: %v", string(bysBody), resp.StatusCode)
	}
	return string(bysBody), nil
}

func (c *Cluster) GetContainers(ctx context.Context, nodeIP string) ([]*containerd.Container, error) {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	api := fmt.Sprintf("http://%v/containers", fmt.Sprintf("%v:8080", nodeIP))

	req, err := http.NewRequest("GET", api, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	bysBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get containers: msg: %v. statusCode: %v", string(bysBody), resp.StatusCode)
	}
	containers := []*containerd.Container{}
	if err := json.Unmarshal(bysBody, &containers); err != nil {
		return nil, err
	}
	return containers, nil
}

// func (c *Cluster) GetContainers(ctx context.Context) (map[string][]*containerd.Container, error) {
// 	client := &http.Client{
// 		Transport: &http.Transport{
// 			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
// 		},
// 	}
// 	m := make(map[string][]*containerd.Container)
// 	for _, node := range c.Nodes {
// 		api := fmt.Sprintf("http://%v/containers", fmt.Sprintf("%v:8080", node.IP))

// 		req, err := http.NewRequest("GET", api, nil)
// 		if err != nil {
// 			return nil, err
// 		}
// 		resp, err := client.Do(req)
// 		if err != nil {
// 			return nil, err
// 		}
// 		bysBody, err := io.ReadAll(resp.Body)
// 		if err != nil {
// 			return nil, err
// 		}
// 		defer resp.Body.Close()
// 		if resp.StatusCode != http.StatusOK {
// 			return nil, fmt.Errorf("failed to get containers: msg: %v. statusCode: %v", string(bysBody), resp.StatusCode)
// 		}
// 		containers := []*containerd.Container{}
// 		if err := json.Unmarshal(bysBody, &containers); err != nil {
// 			return nil, err
// 		}
// 		m[node.IP] = containers
// 	}
// 	return m, nil
// }

type Node struct {
	Interface string     `yaml:"interface"`
	IP        string     `yaml:"ip"`
	VXLAN     *VXLAN     `yaml:"vxlan"`
	Container *Container `yaml:"container"`
}

type Container struct {
	CIDR    string `yaml:"cidr"`
	Gateway string `yaml:"gateway"`
}

type VXLAN struct {
	IP  string `yaml:"ip"`
	MAC string `yaml:"mac"`
}
