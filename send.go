package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

// ListFiles as name says list all files under direcory
func ListFiles(w http.ResponseWriter, req *http.Request) {

	files := []string{}
	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {

		if !info.IsDir() {
			files = append(files, filepath.Base(path))
		}

		return nil
	})
	if err != nil {
		log.Panic(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(files)
}
