#!/usr/bin/env python
from argparse import ArgumentParser
from datetime import datetime
from subprocess import check_output, check_call, CalledProcessError


__VERSION__ = '0.1.2'


date = datetime.now().strftime("%Y-%m-%d_%I:%M:%S%p")
gitVersion = check_output("git rev-parse HEAD", shell=True).decode("utf-8").strip()
goVersion = check_output("go version", shell=True).decode("utf-8").strip()

flags = f"flags='-X main.buildStamp=`{date}` -X main.gitHash=`{gitVersion}` -X main.goVersion=`{goVersion}` -X main.version={__VERSION__} -s -w'"
PLATFORMS = set(["linux", "windows", "darwin", "openbsd"])
ARCHITECTURE = set(["amd64", "arm64", "mips64", "mips64le", "ppc64", "ppc64le", "riscv64", "s390x", "386", "arm", "mips", "mipsle"])


def main():
    parser = ArgumentParser("build transfer from source code")
    parser.add_argument("-p", "--platform", default=None,
                        help="which platform to build, one of {}".format(",".join(PLATFORMS)), type=str)
    parser.add_argument("-a", "--arch", default=None,
                        help="which architecture to build, one of {}".format(",".join(ARCHITECTURE)), type=str)

    args = parser.parse_args()

    platform = PLATFORMS
    arch = ARCHITECTURE

    if args.platform:
        if args.platform.lower() in platform:
            platform = platform & set([args.platform])
        else:
            raise ValueError("{} is not supported".format(args.platform))

    if args.arch:
        if args.arch.lower() in arch:
            arch = arch & set([args.arch])
        else:
            raise ValueError("{} is not supported".format(args.arch))

    for i in platform:
        print(i)

        for j in arch:
            check_call(f"env GOOS='{i}' GOARCH={j} go build -ldflags {flags} -x -o transfer_{i}_{j} .", shell=True)


if __name__ == "__main__":
    main()
