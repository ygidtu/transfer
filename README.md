# transfer

Personal tool used for transfer files between servers.

```bash
Usage: transfer [global options] <verb> [verb options]

Global options:
        -h, --help           Show this help

Verbs:
    get:
        -i, --path           the path to save files
        -h, --host           the target host ip
        -p, --port           the target port
        -x, --proxy          the proxy to use [http or socks5]
    send:
        -i, --path           the path contains files
        -h, --host           the ip address to listern
        -p, --port           the port to listern
    sftp:
        -l, --local          the local path or url
        -h, --host           the remote server [user:passwd@host:port]] (*)
        -r, --remote         remote path in server (*)
            --pull           pull files from server
        -x, --proxy          the proxy to use [socks5 or ssh://user:passwd@host:port]
        -c, --cover          cover old files if exists
            --download       download file and save to server
            --download-proxy the proxy used to download file [socks5 or http]
```

## Usage

```bash
# source server
./transfer send -i path -h 0.0.0.0 -p 8000

# another server
./transfer get -i output -h x.x.x.x -p 8000 -x http://xxxx:port

# push or pull files through sftp
./transfer sftp -l /path/to/local/file --host username:password@x.x.x.x:port -r /path/to/remove/file
# --proxy is the middle server to connect to target server
#  your local -> middle -> reomte target
# --pull is pull files from remote

# just redirct a http request to server
./transfer sftp -l http://to/file --host username:password@x.x.x.x:port -r /path/to/remove/file --download --download-proxy [http or socks5 proxy]
# password is optional, default will try with id_rsa config
```
