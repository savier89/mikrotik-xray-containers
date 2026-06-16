package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
)

//go:embed dist
var distFS embed.FS

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		log.Fatal(err)
	}

	http.Handle("/", http.FileServer(http.FS(sub)))
	log.Printf("Server running on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
