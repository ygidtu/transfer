package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func listFiles() ([]*File, error) {
	var files []*File

	if stat, err := os.Stat(path); os.IsNotExist(err) {
		return files, fmt.Errorf("%s not exists: %v", path, err)
	} else {
		if stat.IsDir() {
			if err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {

				if !info.IsDir() {
					p = strings.ReplaceAll(p, path, "")
					p = strings.TrimLeft(p, "/")
					files = append(files, &File{Path: p, Size: info.Size()})
				}
				return nil
			}); err != nil {
				return files, err
			}
		} else {
			files = append(files, &File{Path: path, Size: stat.Size()})
		}
	}

	return files, nil
}

// ListFiles as name says list all files under direcory
func ListFiles(w http.ResponseWriter, req *http.Request) {

	files, err := listFiles()
	if err != nil {
		log.Error(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(files)
}
