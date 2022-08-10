package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var root *File

/*
##################################
Server
##################################
*/

// ListFiles as name says list all files under directory
func ListFiles(w http.ResponseWriter, _ *http.Request) {

	files, err := ListFilesHTTP(root)
	if err != nil {
		log.Error(err)
	}

	for _, f := range files {
		if f.Path == root.Path {
			f.Path = filepath.Base(f.Path)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(files)
}

// GetFiles as name says get posted file and save it
func GetFiles(w http.ResponseWriter, req *http.Request) {
	var err error
	switch req.Method {
	case "POST":
		{
			var opath string
			mode := "a"
			for k, v := range req.URL.Query() {
				if k == "path" && len(v) > 0 {
					opath = filepath.Join(root.Path, v[0])
				} else if k == "mode" && len(v) > 0 {
					mode = v[0]
				}
			}

			oDir := filepath.Dir(opath)
			if _, err := os.Stat(oDir); os.IsNotExist(err) {
				if err := os.MkdirAll(oDir, os.ModePerm); err != nil {
					e := fmt.Sprintf("failed to create %s: %v", oDir, err)
					log.Error(e)
					break
				}
			}

			var f *os.File
			if mode == "a" {
				f, err = os.OpenFile(opath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, os.ModePerm)
			} else {
				log.Infof("Trunc file %s", opath)
				f, err = os.OpenFile(opath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
			}

			if err != nil {
				e := fmt.Sprintf("failed to open %s: %v", opath, err)
				log.Error(e)
				_, _ = io.WriteString(w, e)
				return
			}

			_, err = io.Copy(f, req.Body)
			if err != nil {
				e := fmt.Sprintf("failed to copy %s: %v", opath, err)
				log.Error(e)
				_, _ = io.WriteString(w, e)
				return
			}
			_ = req.Body.Close()
			_ = f.Close()
			_, _ = io.WriteString(w, "Success")
		}
	case "GET":
		{
			for k, v := range req.URL.Query() {
				if k == "path" && len(v) > 0 {
					opath := filepath.Join(root.Path, v[0])

					if stat, err := os.Stat(opath); !os.IsNotExist(err) {
						_, _ = io.WriteString(w, fmt.Sprintf("%d", stat.Size()))
					} else {
						_, _ = io.WriteString(w, "0")
					}
					break
				}
			}
		}
	}
}

func initServer(opt *options) {
	if opt.Server.Host == "" {
		opt.Server.Host = "0.0.0.0:8000"
	} else {
		if !strings.Contains(opt.Server.Host, ":") {
			opt.Server.Host = fmt.Sprintf("%s:8000", opt.Server.Host)
		}
	}

	if opt.Server.Path == "" {
		opt.Server.Path = "./"
	} else {
		if abs, err := filepath.Abs(opt.Server.Path); err != nil {
			log.Fatal("The input path cannot convert to absolute: %s : %v", opt.Server.Path, err)
		} else {
			opt.Server.Path = abs
		}
	}

	log.Info("path: ", opt.Server.Path)
	log.Info("host: ", opt.Server.Host)

	http.HandleFunc("/list", ListFiles)
	http.HandleFunc("/post", GetFiles)

	if _, ok := os.Stat(opt.Server.Path); os.IsNotExist(ok) {
		if err := os.MkdirAll(opt.Server.Path, os.ModePerm); err != nil {
			log.Fatal(err)
		}
	}
	f, err := NewFile(opt.Server.Path)
	if err != nil {
		log.Fatal(err)
	}
	root = f

	if !f.IsFile {
		fs := http.FileServer(http.Dir(f.Path))
		http.Handle("/", http.StripPrefix("/", fs))
	} else {
		fs := http.FileServer(http.Dir(filepath.Dir(f.Path)))
		http.Handle("/", http.StripPrefix("/", fs))
	}

	log.Error(http.ListenAndServe(opt.Server.Host, nil))
}
