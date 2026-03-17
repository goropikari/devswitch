package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"
)

func main() {
	port := flag.String("port", "9000", "port to listen on")
	flag.Parse()
	addr := ":" + *port

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if _, err := fmt.Fprintf(w, "Hello, World! port = %s\n", *port); err != nil {
			log.Printf("failed to write HTTP response: %v", err)
		}
	})

	log.Printf("Listening on port %s...", *port)
	server := &http.Server{
		Addr:              addr,
		ReadHeaderTimeout: 3 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
