# transfer

Personal tool used for transfer files between servers.

```bash
Usage: transfer [global options] <verb> [verb options]

Global options:
        -h, --help  Show this help

Verbs:
    get:
        -i, --path  the path to save files
        -h, --host  the target host ip
        -p, --port  the target port
        -x, --proxy the proxy to use
    send:
        -i, --path  the path contains files
        -h, --host  the ip address to listern
        -p, --port  the port to listern
```

## Usage

```bash
# source server
./transfer send -i path -h 0.0.0.0 -p 8000

# another server
./transfer get -i output -h x.x.x.x -p 8000 -x http://xxxx:port
```
