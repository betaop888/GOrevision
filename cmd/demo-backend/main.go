package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"
)

func main() {
	port := flag.Int("port", 9001, "port to listen on")
	name := flag.String("name", "backend", "backend name")
	delay := flag.Duration("delay", 0, "artificial response delay")
	flag.Parse()

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "ok\n")
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if *delay > 0 {
			time.Sleep(*delay)
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = fmt.Fprintf(w, "%s handled %s %s\n", *name, r.Method, r.URL.Path)
	})

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("%s listening on %s", *name, addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
