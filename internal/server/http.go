package server

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

var replicas int = 2

// START: newhttpserver
func NewHTTPServer(addr string, serverType string) *http.Server {
	httpsrv := newHTTPServer(serverType)
	r := mux.NewRouter()
	switch httpsrv.ServerType {
	case "master":
		r.HandleFunc("/", httpsrv.handleProduce).Methods("POST")
		r.HandleFunc("/health", httpsrv.handleHealth).Methods("GET")
	case "slave":
		r.HandleFunc("/internal/post", httpsrv.handleProduce).Methods("POST")
	}
	r.HandleFunc("/", httpsrv.handleConsume).Methods("GET")
	go httpsrv.StartHealthChecks()
	return &http.Server{
		Addr:    addr,
		Handler: r,
	}
}

// END: newhttpserver

// START: types
type httpServer struct {
	Log        *Log
	ServerType string
	Agent      *Agent
	Consensus  bool
}

func newHTTPServer(serverType string) *httpServer {
	replicasUrl := newReplicas(replicas)
	return &httpServer{
		Log:        NewLog(),
		ServerType: serverType,
		Agent:      NewAgent(replicasUrl),
		Consensus:  false,
	}
}

func newReplicas(i int) []string {
	var replicas []string
	for k := 1; k <= i; k++ {
		replicas = append(replicas, fmt.Sprintf("slave%d", k))
	}
	return replicas
}

type ProduceRequest struct {
	Record Record `json:"record"`
	W      int    `json:"w,omitempty"`
}

type ReplicateRequest struct {
	Records []Record `json:"records"`
}

type ProduceResponse struct {
	Offset uint64 `json:"offset"`
}

type ConsumeResponse struct {
	Records []Record `json:"records"`
}

type ProduceSecondaryResponse struct {
	StatusCode int
	Error      error
}

// END:types

// START:handleProduce
func (s *httpServer) handleProduce(w http.ResponseWriter, r *http.Request) {
	var produceRequest ProduceRequest

	err := json.NewDecoder(r.Body).Decode(&produceRequest)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	switch s.ServerType {
	case "master":
		if !s.Consensus {
			http.Error(w, "Consensus wasn't reach!", http.StatusNotAcceptable)
			return
		}
		var wg sync.WaitGroup

		produceRequest.Record = s.Log.Append(produceRequest.Record)

		i := produceRequest.W - 1

		replicateRequest := ReplicateRequest{Records: []Record{produceRequest.Record}}

		wg.Add(i)
		for _, replica := range newReplicas(replicas) {
			go s.replicateProduce(&i, &wg, replica, replicateRequest)
		}
		wg.Wait()

		s.writeResponse(produceRequest.Record, w)

	case "slave":
		waitSecs, _ := rand.Int(rand.Reader, big.NewInt(20))
		log.Infof("Waiting %d seconds before responding.", waitSecs)
		time.Sleep(time.Duration(waitSecs.Int64()) * time.Second)
		s.Log.Append(produceRequest.Record)
		s.writeResponse(produceRequest.Record, w)
	}
}

// END:handleProduce

// START:writeResponse
func (s *httpServer) writeResponse(record Record, w http.ResponseWriter) {
	res := ProduceResponse{Offset: record.Offset}
	err := json.NewEncoder(w).Encode(res)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// END:writeResponse

// START:replicateProduce
func (s *httpServer) replicateProduce(i *int, wg *sync.WaitGroup, replica string, replicateRequest ReplicateRequest) {
	timeout := time.After(5 * time.Second)
	for s.Agent.statuses[replica] != healthy {
		select {
		case <-timeout:
			log.WithFields(log.Fields{
				"replica": replica,
			}).Warn("Timeout was reached, aborting!")
			return
		default:
			continue
		}
	}
	jsonValue, _ := json.Marshal(replicateRequest)
	url := fmt.Sprintf("http://%s:9001/internal/post", replica)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonValue))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Warn(err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.WithFields(log.Fields{
			"replica": replica,
		}).Warn("Internal server error")
		go s.replicateProduce(i, wg, replica, replicateRequest)
	} else if resp.StatusCode == http.StatusOK && *i > 0 {
		*i--
		wg.Done()
	}
}

// END:replicateProduce

// START:replicateProduce
func (s *httpServer) replicateSync(replica string) error {
	log.WithFields(log.Fields{
		"replica": replica,
	}).Info("Started replication.")
	for s.Agent.statuses[replica] != healthy {
		time.Sleep(1 * time.Second)
	}
	jsonValue, _ := json.Marshal(ReplicateRequest{Records: s.Log.records})
	fmt.Println(string(jsonValue))
	url := fmt.Sprintf("http://%s:9001/internal/post/sync", replica)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonValue))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return errors.New(resp.Status)
	}
	return nil
}

// END:replicateProduce

// START:consume
func (s *httpServer) handleConsume(w http.ResponseWriter, r *http.Request) {
	records, err := s.Log.Read()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	res := ConsumeResponse{Records: records}
	err = json.NewEncoder(w).Encode(res)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// END:consume

// START:handleHealth
func (s *httpServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	err := json.NewEncoder(w).Encode(s.Agent.statuses)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// END:handleHealth

func (s *httpServer) StartHealthChecks() {
	for {
		for replica := range s.Agent.statuses {
			go s.heartbeat(replica)
		}
		time.Sleep(5 * time.Second)
		i := 1
		for _, status := range s.Agent.statuses {
			if status == healthy {
				i++
			}
		}
		if i > ((replicas + 1) / 2) {
			s.Consensus = true
		} else {
			s.Consensus = false
		}
	}
}

func (s *httpServer) heartbeat(replica string) {
	url := fmt.Sprintf("http://%s:9001/internal/health", replica)
	resp, err := http.Get(url)
	if err != nil {
		switch s.Agent.statuses[replica] {
		case healthy:
			s.Agent.statuses[replica] = suspected
		case suspected:
			log.WithFields(log.Fields{
				"replica": replica,
			}).Warn("Replica is unhealthy!")
			s.Agent.statuses[replica] = unhealthy
		}
		return
	}
	if resp.StatusCode == 200 && s.Agent.statuses[replica] != healthy {
		s.Agent.statuses[replica] = healthy
		log.WithFields(log.Fields{
			"replica": replica,
		}).Info("Replica is healthy!")
		if len(s.Log.records) > 0 {
			err = s.replicateSync(replica)
			if err != nil {
				s.Agent.statuses[replica] = suspected
			}
		}
	}
}
