package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"time"
)

var (
	reqs int
	max  int
)

func init() {
	flag.IntVar(&reqs, "reqs", 60, "Total requests")
	flag.IntVar(&max, "concurrent", 20, "Maximum concurrent requests")
}

type Response struct {
	*http.Response
	err error
}

// Dispatcher
func dispatcher(reqChan chan *http.Request) {
	defer close(reqChan)
	for i := 0; i < reqs; i++ {
		req, err := http.NewRequest("GET", "http://host.containers.internal:8000/", nil)
		if err != nil {
			log.Println(err)
		}
		reqChan <- req
	}
}

// Worker Pool
func workerPool(reqChan chan *http.Request, respChan chan Response) {
	t := &http.Transport{}
	for i := 0; i < max; i++ {
		go worker(t, reqChan, respChan, i)
	}
}

// Worker
func worker(t *http.Transport, reqChan chan *http.Request, respChan chan Response, frame int) {
	for req := range reqChan {
		log.Println("Sending request frame", frame)
		resp, err := t.RoundTrip(req)
		r := Response{resp, err}
		log.Println("Got response frame", frame)
		respChan <- r
	}
}

// Consumer
func consumer(respChan chan Response) (int64, int64) {
	var (
		conns int64
		size  int64
	)
	for conns < int64(reqs) {
		select {
		case r, ok := <-respChan:
			if ok {
				if r.err != nil {
					log.Println(r.err)
				} else {
					size += r.ContentLength
					if err := r.Body.Close(); err != nil {
						log.Println(r.err)
					}
				}
				conns++
			}
		}
	}
	return conns, size
}

func main() {
	flag.Parse()
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	runtime.GOMAXPROCS(runtime.NumCPU())
	reqChan := make(chan *http.Request)
	respChan := make(chan Response)
	start := time.Now()
	go dispatcher(reqChan)
	go workerPool(reqChan, respChan)
	conns, size := consumer(respChan)
	took := time.Since(start)
	ns := took.Nanoseconds()
	av := ns / conns
	average, err := time.ParseDuration(fmt.Sprintf("%d", av) + "ns")
	if err != nil {
		log.Println(err)
	}
	fmt.Printf("Connections:\t%d\nConcurrent:\t%d\nTotal size:\t%d bytes\nTotal time:\t%s\nAverage time:\t%s\n", conns, max, size, took, average)
}
