package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	healthy = iota // c0 == 0
	suspected
	unhealthy
)

type Agent struct {
	statuses map[string]int
}

func NewAgent(replicas []string) *Agent {
	statuses := make(map[string]int)
	for _, replica := range replicas {
		statuses[replica] = suspected
	}
	agent := &Agent{statuses: statuses}
	return agent
}

func (a *Agent) StartHealthChecks() {
	for {
		for replica := range a.statuses {
			go a.heartbeat(replica)
		}
		time.Sleep(5 * time.Second)
	}
}

func (a *Agent) heartbeat(replica string) {
	url := fmt.Sprintf("http://%s:9001/internal/health", replica)
	resp, err := http.Get(url)
	if err != nil {
		logrus.Warn(err)
		switch a.statuses[replica] {
		case healthy:
			a.statuses[replica] = suspected
		case suspected:
			a.statuses[replica] = unhealthy
		}
		return
	}
	if resp.StatusCode == 200 {
		a.statuses[replica] = healthy
	}

}
