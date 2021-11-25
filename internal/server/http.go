package server

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"log"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

var slaves = []string{
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
	W      int8   `json:"w,omitempty"`
}

type ProduceResponse struct {
	Offset uint64 `json:"offset"`
}

type ConsumeResponse struct {
	Records []Record `json:"records"`
}

// END:types

// START:produce
func (s *httpServer) handleProduce(w http.ResponseWriter, r *http.Request) {
	var record Record
	var produceRequest ProduceRequest
	var respErrors []error
	var wg sync.WaitGroup

	e := make(chan error)

	err := json.NewDecoder(r.Body).Decode(&produceRequest)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	record, _ = s.Log.AddOffset(produceRequest.Record)

	switch s.ServerType {
	case "master":
		// If w-value is equal to 1, then don't wait for slaves'
		// responses before writing.
		if produceRequest.W == 1 {
			s.writeLog(record, w)
		}
		go func() {
			for _, url := range slaves {
				wg.Add(1)
				go s.replicateProduce(url, record, e, &wg)
			}
			// Close the channel in the background.
			go func() {
				wg.Wait()
				close(e)
			}()
			// Read from error (e) channel as they come in until its closed.
			for respError := range e {
				respErrors = append(respErrors, respError)
				// If w-value is equal to 2, then wait for one response
				// from any secondary before writing.
				if produceRequest.W == 2 && len(respErrors) == 1 {
					s.writeLog(record, w)
				}
			}
			for _, err := range respErrors {
				if err != nil {
					http.Error(
						w,
						err.Error(),
						http.StatusInternalServerError,
					)
					return
				}
			}
			// If w-value is equal to 3, then wait for responses from both slaves
			// before writing.
			if produceRequest.W == 3 {
				s.writeLog(record, w)
			}
		}()
	case "slave":
		waitSecs, _ := rand.Int(rand.Reader, big.NewInt(20))
		logrus.Infof("Waiting %d seconds before responding.", waitSecs)
		time.Sleep(time.Duration(waitSecs.Int64()) * time.Second)
		s.writeLog(record, w)
	}
}

// END:produce

// START:writeLog
func (s *httpServer) writeLog(record Record, w http.ResponseWriter) {
	s.Log.Append(record)

	res := ProduceResponse{Offset: record.Offset}
	err := json.NewEncoder(w).Encode(res)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

}

// START:replicateProduce
func (s *httpServer) replicateProduce(url string, record Record, errChan chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()
	jsonValue, _ := json.Marshal(record)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonValue))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
		errChan <- err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		errChan <- errors.New(resp.Status)
	}
	errChan <- nil
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
