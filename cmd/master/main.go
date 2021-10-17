package main

import (
	"log"

	"github.com/tooSadman/replicated_log/internal/server"
)

func main() {
	srv := server.NewHTTPServer(":9000", "master")
	log.Fatal(srv.ListenAndServe())
}
