package server

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

var replicas = []string{
	"http://slave1:9001/internal/post",
	"http://slave2:9001/internal/post",
}

// START: newhttpserver
func NewHTTPServer(addr string, serverType string) *http.Server {
	httpsrv := newHTTPServer(serverType)
	r := mux.NewRouter()
	switch httpsrv.ServerType {
	case "master":
		r.HandleFunc("/", httpsrv.handleProduce).Methods("POST")
	case "slave":
		r.HandleFunc("/internal/post", httpsrv.handleProduce).Methods("POST")
	}
	r.HandleFunc("/", httpsrv.handleConsume).Methods("GET")
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
}

func newHTTPServer(serverType string) *httpServer {
	return &httpServer{
		Log:        NewLog(),
		ServerType: serverType,
	}
}

type ProduceRequest struct {
	Record Record `json:"record"`
	W      int    `json:"w,omitempty"`
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
		quit := make(chan bool, produceRequest.W)
		produceRequest.Record = s.Log.Append(produceRequest.Record)

		go s.replicate(replicas, produceRequest, quit)
		for j := 1; j < produceRequest.W; j++ {
			<-quit
		}
		close(quit)
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

// START:replicate
func (s *httpServer) replicate(replicas []string, produceRequest ProduceRequest, quit chan bool) {
	var i int
	e := make(chan ProduceSecondaryResponse)

	for _, url := range replicas {
		go replicateProduce(url, produceRequest.Record, e)
	}
	// Read from error (e) channel as they come in until its closed.
	for resp := range e {
		i++
		if i < produceRequest.W && resp.StatusCode == 200 {
			quit <- true
		}
		fmt.Println(resp)
		//TODO: if resp.StatusCode != 200 { go replicateProduce(...) }
	}
}

// END:replicate

// START:replicateProduce
func replicateProduce(url string, record Record, respChan chan<- ProduceSecondaryResponse) {
	jsonValue, _ := json.Marshal(record)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonValue))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.WithFields(log.Fields{
			"StatusCode": resp.StatusCode,
		}).Warn(err)
		respChan <- ProduceSecondaryResponse{
			StatusCode: resp.StatusCode,
			Error:      err,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respChan <- ProduceSecondaryResponse{
			StatusCode: resp.StatusCode,
			Error:      errors.New(resp.Status),
		}
	}
	respChan <- ProduceSecondaryResponse{
		StatusCode: resp.StatusCode,
		Error:      nil,
	}
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
