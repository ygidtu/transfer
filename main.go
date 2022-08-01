package main

import (
	"github.com/voxelbrain/goptions"
	"go.uber.org/zap"
	"os"
)

var (
	log        *zap.SugaredLogger
	SkipHidden = false
)

// command line parameters
type options struct {
	Skip    bool          `goptions:"--skip, description='Skip hidden files'"`
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
		Path   string `goptions:"-l, --local, description='the local path or url'"`
		Host   string `goptions:"-u, --host, obligatory,description='the remote server [user:passwd@host:port]]'"`
		Remote string `goptions:"-r, --remote, obligatory,description='remote path in server'"`
		Pull   bool   `goptions:"-p, --pull, description='pull files from server'"`
		Proxy  string `goptions:"-x, --proxy, description='the proxy to use [socks5 or ssh://user:passwd@host:port]'"`
		Scp    bool   `goptions:"-s, --scp, description='transfer throught scp instead of sftp'"`
	} `goptions:"sftp"`
	Ftp struct {
		Path   string `goptions:"-l, --local, description='the local path or url'"`
		Host   string `goptions:"-u, --host, obligatory,description='the remote server [user:passwd@host:port]]'"`
		Remote string `goptions:"-r, --remote, obligatory,description='remote path in server'"`
		Pull   bool   `goptions:"--pull, description='pull files from server'"`
	} `goptions:"ftp"`
	Copy struct {
		Path   string `goptions:"-i, --input, description='the source path'"`
		Remote string `goptions:"-o, --output, description='the target path'"`
	} `goptions:"cp"`
}

func main() {
	var options = options{}
	goptions.ParseAndFail(&options)

	if options.Version {
		log.Info("Current version: v0.0.3")
		os.Exit(0)
	}

	SkipHidden = options.Skip

	if options.Verbs == "server" {
		initServer(&options)
	} else if options.Verbs == "trans" {
		initHttp(&options)
	} else if options.Verbs == "sftp" {
		log.Info("Running on sftp mode")
		initSftp(&options)
	} else if options.Verbs == "ftp" {
		initFtp(&options)
	} else if options.Verbs == "cp" {
		initCopy(&options)
	} else {
		goptions.PrintHelp()
	}
}
