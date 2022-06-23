package main

import (
	"fmt"
	"github.com/voxelbrain/goptions"
	"path/filepath"
	"strings"
)

// command line parameters
type options struct {
	Remove  bool          `goptions:"-r, --rm, description='Remove'"`
	Help    goptions.Help `goptions:"-h, --help, description='Show this help'"`
	Version bool          `goptions:"-v, --version, description='Show version information'"`

	goptions.Verbs
	Server struct {
		Path string `goptions:"-i, --path, description='the path contains files'"`
		Host string `goptions:"-u, --host, description='the ip address to listen [ip:port]'"`
	} `goptions:"server"`
	Trans struct {
		Path  string `goptions:"-i, --path, description='the path to save files'"`
		Host  string `goptions:"-u, --host, description='the target host [ip:port]'"`
		Proxy string `goptions:"-x, --proxy, description='the proxy to use [http or socks5]'"`
		Post  bool   `goptions:"-p, --post, description='the proxy to use [http or socks5]'"`
	} `goptions:"trans"`
	Sftp struct {
		Path     string `goptions:"-l, --local, description='the local path or url'"`
		Host     string `goptions:"-u, --host, obligatory,description='the remote server [user:passwd@host:port]]'"`
		Remote   string `goptions:"-r, --remote, obligatory,description='remote path in server'"`
		Pull     bool   `goptions:"-p, --pull, description='pull files from server'"`
		Proxy    string `goptions:"-x, --proxy, description='the proxy to use [socks5 or ssh://user:passwd@host:port]'"`
		Scp      bool   `goptions:"-s, --scp, description='transfer throught scp instead of sftp'"`
		Download bool   `goptions:"--download, description='download file and save to server'"`
		ProxyD   string `goptions:"--download-proxy, description='the proxy used to download file [socks5 or http]'"`
	} `goptions:"sftp"`
	Ftp struct {
		Path   string `goptions:"-l, --local, description='the local path or url'"`
		Host   string `goptions:"-u, --host, obligatory,description='the remote server [user:passwd@host:port]]'"`
		Remote string `goptions:"-r, --remote, obligatory,description='remote path in server'"`
		Pull   bool   `goptions:"--pull, description='pull files from server'"`
	} `goptions:"ftp"`
}

// process and set send options
func defaultSend(opt *options) {
	if opt.Server.Host == "" {
		host = "0.0.0.0:8000"
	} else {
		host = opt.Server.Host

		if !strings.Contains(opt.Server.Host, ":") {
			host = fmt.Sprintf("%s:8000", host)
		}
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
}

// process and set get options
func defaultGet(opt *options) {
	if opt.Trans.Host == "" {
		host = "127.0.0.1:8000"
	} else {
		host = opt.Trans.Host

		if !strings.Contains(opt.Trans.Host, ":") {
			host = fmt.Sprintf("%s:8000", host)
		}
	}
	if !strings.HasPrefix(host, "http") {
		host = fmt.Sprintf("http://%s", host)
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
}
