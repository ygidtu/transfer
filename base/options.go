package base

import "github.com/voxelbrain/goptions"

// Options command line parameters
type Options struct {
	Source     string        `goptions:"-i, --input, obligatory, description='the source file path;\n\t\t\t\tthe remote path should be [http|ftp|ssh|s3]://user:password@ip:port/path'"`
	Target     string        `goptions:"-o, --output, description='the target file path;\n\t\t\t\tthe remote path should be [http|ftp|ssh|s3]://user:password@ip:port/path'"`
	Server     string        `goptions:"-s, --server, description='the server host url and port'"`
	Proxy      string        `goptions:"-x, --proxy, description='the proxy to use [http, socks5 or ssh://user:passwd@host:port]; \n\t\t\t\tthe http support http/socks5 proxy\n\t\t\t\tthe ssh support socks5 and ssh proxy\n\t\t\t\tthe aws s3 support http proxy'"`
	Bucket     string        `goptions:"-b, --bucket, description='the bucket name of aws s3, use first bucket as default in buckets list'"`
	Scp        bool          `goptions:"-s, --scp, description='transfer through scp instead of sftp'"`
	IdRsa      string        `goptions:"-I, --rsa, description='path to id_rsa file, default: ~/.ssh/id_rsa'"`
	Concurrent int           `goptions:"-n, --n-jobs, description='the number of jobs to run'"`
	Daemon     bool          `goptions:"-d, --daemon, description='run transfer in daemon mode'"`
	Skip       bool          `goptions:"--skip, description='skip hidden files'"`
	Help       goptions.Help `goptions:"-h, --help, description='show this help'"`
	Version    bool          `goptions:"-v, --version, description='show version information'"`
	Debug      bool          `goptions:"--debug, description='show more info'"`
}
