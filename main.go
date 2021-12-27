package main

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/voxelbrain/goptions"
	"go.uber.org/zap"
)

var (
	path      string
	host      string
	port      int
	transport *http.Transport

	// Sugar is zap sugared logger
	log *zap.SugaredLogger
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
		proxyURL, err := url.Parse(opt.Get.Proxy)
		if err != nil {
			log.Fatal(err, "Proxy format error")
		}

		transport = &http.Transport{
			Proxy:               http.ProxyURL(proxyURL),
			MaxIdleConns:        1,
			MaxIdleConnsPerHost: 1,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
		}
	}
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

		for _, u := range targets {
			if err := Download(fmt.Sprintf("%v:%v/%v", host, port, url.PathEscape(u)), filepath.Join(path, u)); err != nil {
				log.Warn(err)
			}
		}
	}
}
