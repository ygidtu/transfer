package main

import (
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/voxelbrain/goptions"
	"go.uber.org/zap"
)

var (
	path  string
	host  string
	port  int
	log   *zap.SugaredLogger
	proxy *url.URL
)

// comand line parameters
type options struct {
	Help goptions.Help `goptions:"-h, --help, description='Show this help'"`

	goptions.Verbs
	Send struct {
		Path string `goptions:"-i, --path, description='the path contains files'"`
		Host string `goptions:"-h, --host, description='the ip address to listern'"`
		Port int    `goptions:"-p, --port, description='the port to listern'"`
	} `goptions:"send"`
	Get struct {
		Path  string `goptions:"-i, --path, description='the path to save files'"`
		Host  string `goptions:"-h, --host, description='the target host ip'"`
		Port  int    `goptions:"-p, --port, description='the target port'"`
		Proxy string `goptions:"-x, --proxy, description='the proxy to use'"`
	} `goptions:"get"`
	Sftp struct {
		Path     string `goptions:"-l, --local, description='the local path'"`
		Host     string `goptions:"-h, --host, obligatory,description='the ip address to listern'"`
		Port     int    `goptions:"-p, --port, description='the port of ssh [default: 22]'"`
		Remote   string `goptions:"-r, --remote, obligatory,description='remote path in server'"`
		Username string `goptions:"-u, --user, obligatory,description='the username'"`
		Passwd   string `goptions:"-w, --password, description='the password of user [optional, default will try with id_rsa config]'"`
		Pull     bool   `goptions:"--pull, description='pull files from server'"`
	} `goptions:"sftp"`
}

// process and set send options
func defaultSend(opt options) {
	if opt.Send.Host == "" {
		host = "0.0.0.0"
	} else {
		host = opt.Send.Host
	}

	if opt.Send.Path == "" {
		path = "./"
	} else {
		path = opt.Send.Path
	}

	if opt.Send.Port == 0 {
		port = 8000
	} else {
		port = opt.Send.Port
	}
}

// process and set get options
func defaultGet(opt options) {
	if opt.Get.Host == "" {
		host = "127.0.0.1"
	} else {
		host = opt.Get.Host
	}

	if !strings.HasPrefix(host, "http") {
		host = fmt.Sprintf("http://%v", host)
	}

	if opt.Get.Path == "" {
		path = "./"
	} else {
		path = opt.Get.Path
	}

	if opt.Get.Port == 0 {
		port = 8000
	} else {
		port = opt.Get.Port
	}

	if opt.Get.Proxy != "" {
		p, err := url.Parse(opt.Get.Proxy)
		if err != nil {
			log.Fatal(err, "Proxy format error")
		}

		proxy = p
	}
}

// process and set push options
func defaultSftp(opt options) {
	host = opt.Sftp.Host

	if opt.Sftp.Path == "" {
		opt.Sftp.Path = "./"
	}

	if opt.Sftp.Pull {
		path = opt.Sftp.Remote
	} else {
		path = opt.Sftp.Path
	}

	if opt.Sftp.Port == 0 {
		port = 22
	} else {
		port = opt.Sftp.Port
	}
}

type File struct {
	Path string
	Size int64
}

func main() {
	var options = options{}
	goptions.ParseAndFail(&options)

	if options.Verbs == "send" {
		defaultSend(options)

		log.Info("path: ", path)
		log.Info("host: ", host)
		log.Info("port: ", port)

		http.HandleFunc("/list", ListFiles)

		fs := http.FileServer(http.Dir(path))
		http.Handle("/", http.StripPrefix("/", fs))

		log.Error(http.ListenAndServe(fmt.Sprintf("%v:%v", host, port), nil))
	} else if options.Verbs == "get" {
		defaultGet(options)

		log.Info("path: ", path)
		log.Info("host: ", host)
		log.Info("port: ", port)

		targets, err := GetList()
		if err != nil {
			log.Error(err)
		}

		for idx, file := range targets {
			log.Infof("[%d/%d] start to download: %v", idx+1, len(targets), file.Path)
			if err := Download(file); err != nil {
				log.Warn(err)
			}
		}
	} else if options.Verbs == "sftp" {
		defaultSftp(options)

		if options.Sftp.Pull {
			log.Infof("Pull %s@%s:%d:%s to %s", options.Sftp.Username, host, port, options.Sftp.Remote, options.Sftp.Path)
		} else {
			log.Infof("Push %s to %s@%s:%d:%s", options.Sftp.Path, options.Sftp.Username, host, port, options.Sftp.Remote)
		}

		client := &ClientConfig{
			Host:     host,
			Port:     port,
			Username: options.Sftp.Username,
			Password: options.Sftp.Passwd,
		}

		if err := client.Connect(); err != nil {
			log.Fatal(err)
		}

		// check whether target is exists
		if err := client.Mkdir(options.Sftp.Remote); err != nil {
			log.Fatal(err)
		}

		var files []File
		if options.Sftp.Pull {
			fs, err := client.GetFiles(options.Sftp.Remote, options.Sftp.Pull)
			if err != nil {
				log.Fatal(err)
			}
			files = append(files, fs...)
		} else {
			fs, err := client.GetFiles(options.Sftp.Path, options.Sftp.Pull)
			if err != nil {
				log.Fatal(err)
			}
			files = append(files, fs...)
		}

		for idx, f := range files {
			log.Infof("[%d/%d] %s", idx+1, len(files), f.Path)

			if options.Sftp.Pull {
				if err := client.Download(f, filepath.Join(options.Sftp.Path, f.Path)); err != nil {
					log.Warn(err)
				}
			} else {
				if err := client.Upload(f, filepath.Join(options.Sftp.Remote, f.Path)); err != nil {
					log.Warn(err)
				}
			}
		}

		defer client.sftpClient.Close()
		defer client.sshClient.Close()
	}
}
