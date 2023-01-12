flags="-X main.buildStamp=`date -u '+%Y-%m-%d_%I:%M:%S%p'` -X main.gitHash=`git rev-parse HEAD` -X 'main.goVersion=`go version`' -X main.version=v0.1.1"

env GOOS=linux GOARCH=amd64 go build -ldflags "$flags" -x -o transfer_linux_amd64 .
env GOOS=darwin GOARCH=amd64 go build -ldflags "$flags" -x -o transfer_darwin_amd64 .
env GOOS=windows GOARCH=amd64 go build -ldflags "$flags" -x -o transfer_win_amd64.exe .
