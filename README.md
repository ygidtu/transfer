# transfer

Personal tool used for transfer files between servers.

```bash
transfer  --help
Usage: transfer [global options] <verb> [verb options]

Global options:
            --skip    Skip hidden files
        -h, --help    Show this help
        -v, --version Show version information

Verbs:
    cp:
        -i, --input   the source path
        -o, --output  the target path
    ftp:
        -l, --local   the local path or url
        -u, --host    the remote server [user:passwd@host:port]] (*)
        -r, --remote  remote path in server (*)
            --pull    pull files from server
    server:
        -i, --path    the path contains files
        -u, --host    the ip address to listen [ip:port]
    sftp:
        -l, --local   the local path or url
        -u, --host    the remote server [user:passwd@host:port]] (*)
        -r, --remote  remote path in server (*)
        -p, --pull    pull files from server
        -x, --proxy   the proxy to use [socks5 or ssh://user:passwd@host:port]
        -s, --scp     transfer throught scp instead of sftp
    trans:
        -i, --path    the path to save files
        -u, --host    the target host [ip:port]
        -x, --proxy   the proxy to use [http or socks5]
        -p, --post    the proxy to use [http or socks5]
```

## Usage

```bash
# source server
./transfer server -i path -h 0.0.0.0 -p 8000

# another server
./transfer trans -i output -h x.x.x.x -p 8000 -x http://xxxx:port

# push or pull files through sftp
./transfer sftp -l /path/to/local/file --host username:password@x.x.x.x:port -r /path/to/remove/file
# --proxy is the middle server to connect to target server
#  your local -> middle -> remote target
# --pull is pull files from remote

./transfer sftp -l /path/to/local/file --host username:password@x.x.x.x:port --scp -r /path/to/remove/file
# this will transfer file through scp, more faster but cannot resume the progress
```

