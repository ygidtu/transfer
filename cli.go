package main

import (
	"fmt"
	"github.com/voxelbrain/goptions"
	"path/filepath"
	"strings"
)

// command line parameters
type options struct {
	Help goptions.Help `goptions:"-h, --help, description='Show this help'"`

	goptions.Verbs
	Server struct {
		Path string `goptions:"-i, --path, description='the path contains files'"`
		Host string `goptions:"-h, --host, description='the ip address to listen'"`
		Port int    `goptions:"-p, --port, description='the port to listen'"`
	} `goptions:"server"`
	Trans struct {
		Path    string `goptions:"-i, --path, description='the path to save files'"`
		Host    string `goptions:"-h, --host, description='the target host ip'"`
		Port    int    `goptions:"-p, --port, description='the target port'"`
		Proxy   string `goptions:"-x, --proxy, description='the proxy to use [http or socks5]'"`
		Post    bool   `goptions:"-s, --post, description='the proxy to use [http or socks5]'"`
		Threads int    `goptions:"-t, --threads, description='the threads to use'"`
	} `goptions:"trans"`
	Sftp struct {
		Path     string `goptions:"-l, --local, description='the local path or url'"`
		Host     string `goptions:"-h, --host, obligatory,description='the remote server [user:passwd@host:port]]'"`
		Remote   string `goptions:"-r, --remote, obligatory,description='remote path in server'"`
		Pull     bool   `goptions:"--pull, description='pull files from server'"`
		Proxy    string `goptions:"-x, --proxy, description='the proxy to use [socks5 or ssh://user:passwd@host:port]'"`
		Scp      bool   `goptions:"-s, --scp, description='transfer throught scp instead of sftp'"`
		Download bool   `goptions:"--download, description='download file and save to server'"`
		ProxyD   string `goptions:"--download-proxy, description='the proxy used to download file [socks5 or http]'"`
		Threads  int    `goptions:"-t, --threads, description='the threads to use'"`
	} `goptions:"sftp"`
	Ftp struct {
		Path    string `goptions:"-l, --local, description='the local path or url'"`
		Host    string `goptions:"-h, --host, obligatory,description='the remote server [user:passwd@host:port]]'"`
		Remote  string `goptions:"-r, --remote, obligatory,description='remote path in server'"`
		Pull    bool   `goptions:"--pull, description='pull files from server'"`
		Threads int    `goptions:"-t, --threads, description='the threads to use'"`
	} `goptions:"ftp"`
}

// process and set send options
func defaultSend(opt *options) {
	if opt.Server.Host == "" {
		host = "0.0.0.0"
	} else {
		host = opt.Server.Host
	}

	if opt.Server.Path == "" {
		path = "./"
	} else {
		if abs, err := filepath.Abs(opt.Server.Path); err != nil {
			log.Fatal("The input path cannot convert to absolute: %s : %v", opt.Server.Path, err)
		} else {
			path = abs
		}
	}

	if opt.Server.Port == 0 {
		port = 8000
	} else {
		port = opt.Server.Port
	}
}

// process and set get options
func defaultGet(opt *options) {
	if opt.Trans.Host == "" {
		host = "127.0.0.1"
	} else {
		host = opt.Trans.Host
	}

	if !strings.HasPrefix(host, "http") {
		host = fmt.Sprintf("http://%v", host)
	}

	if opt.Trans.Path == "" {
		path = "./"
	} else {
		if abs, err := filepath.Abs(opt.Trans.Path); err != nil {
			log.Fatal("The input path cannot convert to absolute: %s : %v", opt.Trans.Path, err)
		} else {
			path = abs
		}
	}

	if opt.Trans.Port == 0 {
		port = 8000
	} else {
		port = opt.Trans.Port
	}

	if opt.Trans.Threads == 0 {
		opt.Trans.Threads = 1
	}

	if opt.Trans.Proxy != "" {
		p, err := CreateProxy(opt.Trans.Proxy)
		if err != nil {
			log.Fatal(err, "proxy format error")
		}

		proxy = p

		if p.Scheme != "http" && p.Scheme != "socks5" {
			log.Fatalf("http get mode do not support this kind of proxy: %s", p.Scheme)
		}
	}
}

// process and set push options
func defaultSftp(opt *options) {
	host = opt.Sftp.Host
	if !strings.HasPrefix(host, "ssh") {
		host = fmt.Sprintf("ssh://%s", host)
	}

	if opt.Sftp.Path == "" {
		opt.Sftp.Path = "./"
	}

	if opt.Sftp.Pull {
		path = opt.Sftp.Remote
	} else {
		if abs, err := filepath.Abs(opt.Sftp.Path); err != nil {
			log.Fatal("The input path cannot convert to absolute: %s : %v", opt.Sftp.Path, err)
		} else {
			path = abs
		}
	}

	if opt.Sftp.Proxy != "" {
		p, err := CreateProxy(opt.Sftp.Proxy)
		if err != nil {
			log.Fatal(err, "proxy format error")
		}

		sftpProxy = p

		if p.Scheme != "ssh" && p.Scheme != "socks5" {
			log.Fatalf("sftp mode do not support this kind of proxy: %s", p.Scheme)
		}
	}

	if opt.Sftp.ProxyD != "" {
		p, err := CreateProxy(opt.Sftp.ProxyD)
		if err != nil {
			log.Fatal(err, "proxy format error")
		}

		proxy = p

		if p.Scheme != "http" && p.Scheme != "socks5" {
			log.Fatalf("the download proxy do not support this kind of proxy: %s", p.Scheme)
		}
	}

	if opt.Sftp.Threads == 0 {
		opt.Sftp.Threads = 1
	}
}

func defaultFtp(opt *options) {
	host = opt.Ftp.Host

	if opt.Ftp.Path == "" {
		opt.Ftp.Path = "./"
	}

	if abs, err := filepath.Abs(opt.Ftp.Path); err != nil {
		log.Fatal("The input path cannot convert to absolute: %s : %v", opt.Ftp.Path, err)
	} else {
		path = abs
	}

	log.Info(path)

	if opt.Ftp.Threads == 0 {
		opt.Ftp.Threads = 1
	}
}