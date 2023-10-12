flags="-X main.buildStamp=`date -u '+%Y-%m-%d_%I:%M:%S%p'` -X main.gitHash=`git rev-parse HEAD` -X 'main.goVersion=`go version`' -X main.version=v0.1.1"

declare -a arr=(linux darwin windows)

## now loop through the above array
for i in "${arr[@]}"
do
   echo "$i"
  env GOOS="$i" GOARCH=amd64 go build -ldflags "$flags -s -w" -x -o transfer_$i_amd64 . && upx -9 transfer_$i_amd64
done