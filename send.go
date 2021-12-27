package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// ListFiles as name says list all files under direcory
func ListFiles(w http.ResponseWriter, req *http.Request) {

	files := []File{}
	err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {

		if !info.IsDir() {
			p = strings.ReplaceAll(p, path, "")
			p = strings.TrimLeft(p, "/")

			files = append(files, File{Path: p, Size: info.Size()})
		}

		return nil
	})
	if err != nil {
		log.Error(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(files)
}
