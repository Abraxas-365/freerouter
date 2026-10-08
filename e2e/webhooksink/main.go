// Command webhooksink records webhook deliveries for the end-to-end suite.
//
//	POST /hook/<any>            records the delivery, replies 200
//	POST /hook/fail/<n>/<any>   replies 500 for the first n deliveries to that
//	                            exact path, then 200 (exercises retry)
//	GET  /_deliveries           lists recorded deliveries (newest last)
//	DELETE /_deliveries         clears recordings and failure counters
//	GET  /_health               readiness probe
package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type delivery struct {
	At      time.Time         `json:"at"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
	Body    json.RawMessage   `json:"body"`
	Status  int               `json:"status"`
}

type sink struct {
	mu         sync.Mutex
	deliveries []delivery
	failures   map[string]int
}

func main() {
	addr := ":" + os.Getenv("PORT")
	if addr == ":" {
		addr = ":9200"
	}
	s := &sink{failures: map[string]int{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/_health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	mux.HandleFunc("/_deliveries", s.handleList)
	mux.HandleFunc("/hook/", s.handleHook)
	log.Printf("webhooksink listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func (s *sink) handleList(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.Method == http.MethodDelete {
		s.deliveries = nil
		s.failures = map[string]int{}
		w.WriteHeader(204)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.deliveries)
}

func (s *sink) handleHook(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	headers := map[string]string{}
	for k, v := range r.Header {
		headers[k] = strings.Join(v, ",")
	}
	status := 200
	if strings.HasPrefix(r.URL.Path, "/hook/fail/") {
		rest := strings.TrimPrefix(r.URL.Path, "/hook/fail/")
		n, _ := strconv.Atoi(strings.SplitN(rest, "/", 2)[0])
		s.mu.Lock()
		seen := s.failures[r.URL.Path]
		s.failures[r.URL.Path] = seen + 1
		s.mu.Unlock()
		if seen < n {
			status = 500
		}
	}
	d := delivery{At: time.Now(), Path: r.URL.Path, Headers: headers, Status: status}
	if json.Valid(body) {
		d.Body = body
	} else {
		d.Body, _ = json.Marshal(string(body))
	}
	s.mu.Lock()
	s.deliveries = append(s.deliveries, d)
	s.mu.Unlock()
	w.WriteHeader(status)
}
