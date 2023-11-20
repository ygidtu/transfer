package base

import (
	"fmt"
	"github.com/DavidGamba/go-getoptions"
	"os"
	"path/filepath"
)

// Options command line parameters
type Options struct {
	Source     string
	Target     string
	Server     string
	Proxy      string
	Bucket     string
	Scp        bool
	IdRsa      string
	Concurrent int
	Daemon     bool
	Skip       bool
	Help       bool
	Version    bool
	Debug      bool

	opt *getoptions.GetOpt
}

func InitOptions() *Options {
	opt := &Options{opt: getoptions.New()}

	setLogger(false)

	dirname, err := os.UserHomeDir()
	if err != nil {
		SugaredLog.Fatal(err)
	}

	opt.opt.BoolVar(&opt.Help, "help", false, opt.opt.Alias("h"),
		opt.opt.Description("show help information"))
	opt.opt.BoolVar(&opt.Debug, "debug", false,
		opt.opt.Description("show more info"))
	opt.opt.BoolVar(&opt.Version, "version", false, opt.opt.Alias("v"),
		opt.opt.Description("show version information"))
	opt.opt.BoolVar(&opt.Skip, "skip", false,
		opt.opt.Description("skip hidden file"))
	opt.opt.BoolVar(&opt.Daemon, "daemon", false, opt.opt.Alias("d"),
		opt.opt.Description("run transfer in daemon mode"))
	opt.opt.BoolVar(&opt.Scp, "scp", false,
		opt.opt.Description("transfer through scp instead of sftp"))
	opt.opt.StringVar(&opt.Source, "input", "", opt.opt.Alias("i"),
		opt.opt.Description("the source file path;\nthe remote path should be [http|ftp|ssh|s3]://user:password@ip:port/path"))
	opt.opt.StringVar(&opt.Target, "output", "", opt.opt.Alias("o"),
		opt.opt.Description("the target file path;\nthe remote path should be [http|ftp|ssh|s3]://user:password@ip:port/path"))
	opt.opt.StringVar(&opt.Server, "server", "", opt.opt.Alias("s"),
		opt.opt.Description("the server host url and port"))
	opt.opt.StringVar(&opt.Proxy, "proxy", "", opt.opt.Alias("x"),
		opt.opt.Description("the proxy to use [http, socks5 or ssh://user:passwd@host:port]; \nthe http support http/socks5 proxy\nthe ssh support socks5 and ssh proxy\nthe aws s3 support http proxy"))
	opt.opt.StringVar(&opt.Bucket, "bucket", "", opt.opt.Alias("b"),
		opt.opt.Description("the bucket name of aws s3, use first bucket as default in buckets lis"))
	opt.opt.StringVar(&opt.IdRsa, "rsa", filepath.Join(dirname, ".ssh/id_rsa"), opt.opt.Alias("r"),
		opt.opt.Description("path to id_rsa file"))
	opt.opt.IntVar(&opt.Concurrent, "n-jobs", 1, opt.opt.Alias("n"),
		opt.opt.Description("number of threads to use"))

	remaining, err := opt.opt.Parse(os.Args[1:])
	if err != nil {
		SugaredLog.Fatal(err)
	}

	if len(remaining) > 0 {
		if len(opt.Source) < 1 {
			opt.Source = remaining[0]
		}

		if opt.Target == "" {
			opt.Target = remaining[len(remaining)-1]
		}
	}

	if opt.Debug {
		setLogger(opt.Debug)
	}

	if opt.opt.Called("help") || len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, opt.opt.Help())
		os.Exit(1)
	}

	return opt
}
