package base

import "github.com/voxelbrain/goptions"

// Options command line parameters
type Options struct {
	Concurrent int           `goptions:"-n, --n-jobs, description='the number of jobs to run'"`
	Skip       bool          `goptions:"--skip, description='skip hidden files'"`
	Help       goptions.Help `goptions:"-h, --help, description='show this help'"`
	Pull       bool          `goptions:"-p, --pull, description='pull files from server'"`
	Version    bool          `goptions:"-v, --version, description='show version information'"`
	Debug      bool          `goptions:"-d, --debug, description='show more info'"`

	goptions.Verbs
	Http struct {
		Path   string `goptions:"-i, --path, description='the path to save files'"`
		Host   string `goptions:"-u, --host, description='the target host [ip:port]'"`
		Proxy  string `goptions:"-x, --proxy, description='the proxy to use [http or socks5]'"`
		Server string `goptions:"-s, --server, description='the server mode'"`
	} `goptions:"http"`
	Sftp struct {
		Path string `goptions:"-l, --local, description='the local path or url'"`
		Host string `goptions:"-u, --host, obligatory,description='the remote server [user:passwd@host:port]]'"`
		//Target string `goptions:"-t, --target, description='the remote server [user:passwd@host:port]], used to transfer data from --host to this one'"`
		Remote string `goptions:"-r, --remote, obligatory,description='remote path in server'"`
		Proxy  string `goptions:"-x, --proxy, description='the proxy to use [socks5 or ssh://user:passwd@host:port]'"`
		Scp    bool   `goptions:"-s, --scp, description='transfer through scp instead of sftp'"`
		IdRsa  string `goptions:"-i, --rsa, description='path to id_rsa file, default: ~/.ssh/id_rsa'"`
	} `goptions:"sftp"`
	Ftp struct {
		Path   string `goptions:"-l, --local, description='the local path or url'"`
		Host   string `goptions:"-u, --host, obligatory,description='the remote server [user:passwd@host:port]]'"`
		Remote string `goptions:"-r, --remote, obligatory,description='remote path in server'"`
	} `goptions:"ftp"`
	Copy struct {
		Path   string `goptions:"-i, --input, description='the source path'"`
		Remote string `goptions:"-o, --output, description='the target path'"`
	} `goptions:"cp"`
}
