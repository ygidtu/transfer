# transfer

Personal tool used for transfer files between servers.

```bash
Usage: transfer [global options] <verb> [verb options]

Global options:
        -h, --help  Show this help

Verbs:
    get:
        -i, --path     the path to save files
        -h, --host     the target host ip
        -p, --port     the target port
        -x, --proxy    the proxy to use
    send:
        -i, --path     the path contains files
        -h, --host     the ip address to listern
        -p, --port     the port to listern
    sftp:
        -l, --local    the local path
        -h, --host     the ip address to listern (*)
        -p, --port     the port of ssh [default: 22]
        -r, --remote   remote path in server (*)
        -u, --user     the username (*)
        -w, --password the password of user[optional, default will try with id_rsa config]
            --pull     pull files from server
```

## Usage

```bash
# source server
./transfer send -i path -h 0.0.0.0 -p 8000

# another server
./transfer get -i output -h x.x.x.x -p 8000 -x http://xxxx:port

# push or pull files through sftp
./transfer sftp -l /path/to/local/file -h x.x.x.x --user username -r /path/to/remove/file --password password[optional, default will try with id_rsa config]
```
