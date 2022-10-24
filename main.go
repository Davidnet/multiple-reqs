package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"time"
	// "io"
)

type Data struct {
	Image            string `json:"image"`
	Lighting         string `json:"lighting"`
	Frameid          uint   `json:"frameid"`
	Format           string `json:"format"`
	Width            uint   `json:"width"`
	Height           uint   `json:"height"`
	Not_brightness   int    `json:"not_brightness"`
	Do_tracking      int    `json:"do_tracking"`
	Tracking_delta_y int    `json:"tracking_delta_y"`
	Tracking_state   string `json:"tracking_state"`
}

var (
	reqs int
	max  int
)

func init() {
	flag.IntVar(&reqs, "reqs", 30*2, "Total requests")
	flag.IntVar(&max, "concurrent", 31, "Maximum concurrent requests")
}

type Response struct {
	*http.Response
	err error
}

type Request struct {
	*http.Request
	frame int
}

// Dispatcher
func dispatcher(reqChan chan Request) {
	defer close(reqChan)
	content, err := os.ReadFile("./data/test.json")
	if err != nil {
		log.Fatal("Error when opening file: ", err)
	}
	var payload Data
	err = json.Unmarshal(content, &payload)
	if err != nil {
		log.Fatal("Error during Unmarshal(): ", err)
	}
	data := url.Values{}
	data.Set("image", payload.Image)
	data.Set("lighting", payload.Lighting)
	data.Set("format", payload.Format)
	data.Set("width", fmt.Sprintf("%d", payload.Width))
	data.Set("height", fmt.Sprintf("%d", payload.Height))
	data.Set("not_brightness", fmt.Sprintf("%d", payload.Not_brightness))
	data.Set("do_tracking", fmt.Sprintf("%d", payload.Do_tracking))
	data.Set("tracking_delta_y", fmt.Sprintf("%d", payload.Tracking_delta_y))
	data.Set("tracking_state", payload.Tracking_state)

	for i := 0; i < reqs; i++ {
		data.Set("frameid", fmt.Sprint(i))
		req, err := http.NewRequest("POST", "http://host.containers.internal:8000/detect", bytes.NewBufferString(data.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if err != nil {
			log.Println(err)
		}
		reqChan <- Request{req, i}
	}
}

// Worker Pool
func workerPool(reqChan chan Request, respChan chan Response) {
	t := &http.Transport{}
	for i := 0; i < max; i++ {
		go worker(t, reqChan, respChan)
	}
}

// Worker
func worker(t *http.Transport, reqChan chan Request, respChan chan Response) {

	for req := range reqChan {
		log.Println("Sending request frame", req.frame)
		resp, err := t.RoundTrip(req.Request)
		log.Println("Received response frame", req.frame)
		r := Response{resp, err}
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
					// log.Println("Status code:", r.StatusCode)
					// inference, err := io.ReadAll(r.Body)
					// if err != nil {
					//     log.Fatalln(err)
					// }
					// log.Println(string(inference))
					// if err := r.Body.Close(); err != nil {
					// 	log.Println(r.err)
					// }
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
	reqChan := make(chan Request)
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
