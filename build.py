#!/usr/bin/env python

from datetime import datetime
from subprocess import check_output, check_call, CalledProcessError


__VERSION__ = '0.1.2'


date = datetime.now().strftime("%Y-%m-%d_%I:%M:%S%p")
gitVersion = check_output("git rev-parse HEAD", shell=True).decode("utf-8").strip()
goVersion = check_output("go version", shell=True).decode("utf-8").strip()

flags = f"flags='-X main.buildStamp=`{date}` -X main.gitHash=`{gitVersion}` -X main.goVersion=`{goVersion}` -X main.version={__VERSION__} -s -w'"

for i in ["linux", "windows", "darwin"]:
    print(i)

    check_call(f"env GOOS='{i}' GOARCH=amd64 go build -ldflags {flags} -x -o transfer_{i}_amd64 .", shell=True)
    if i != "linux":
        try:
            check_call(f"upx -9 transfer_{i}_amd64", shell=True)
        except CalledProcessError as e:
            continue
