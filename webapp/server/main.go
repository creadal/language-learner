package main

import (
	"log"
	"net/http"
)

func main() {
	server := newServer("postgres://postgres:1111@localhost:5432/lang")
	server.createRouter()

	log.Fatal(http.ListenAndServe("0.0.0.0:8080", server.router))
}
