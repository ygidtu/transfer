# transfer

Personal tool used for transfer files between servers.

---

## 1. Installation

### 1.1 pre-build binary

Please download the pre-build binary from [releases page](https://github.com/ygidtu/transfer/releases) according to your system and cpu platform

### 1.2 compile from source

1. first, please install golang (v.1.3+) according the [official instructions](https://go.dev/doc/install)
   - 国内用户则可从[study golang](https://studygolang.com/dl)下载并按照安装说明安装 
2. second, for users have trouble with download golang packages, please set up GOPROXY following [tutorial](https://goproxy.io/)
3. compile the binary executable by the following code:
   ```bash
   # pull source code from this repo
   git pull git@github.com:ygidtu/transfer.git && cd transfer
   
   # build the binary names transfer 
   go build -o transfer . 
   
   # execute transfer to check the help information
   ./transfer --help
   ```

---

## 2. Usage

### 2.1 Help message

```bash
Usage: transfer [global options] 

Global options:
        -i, --input   the source file path;
                      the remote path should be [http|ftp|ssh|s3]://user:password@ip:port/path (*)
        -o, --output  the target file path;
                      the remote path should be [http|ftp|ssh|s3]://user:password@ip:port/path
        -s, --server  the server host url and port
        -x, --proxy   the proxy to use [http, socks5 or ssh://user:passwd@host:port]; 
                      the http support http/socks5 proxy
                      the ssh support socks5 and ssh proxy
                      the aws s3 support http proxy
        -b, --bucket  the bucket name of aws s3, use first bucket as default in buckets list
        -s, --scp     transfer through scp instead of sftp
        -I, --rsa     path to id_rsa file, default: ~/.ssh/id_rsa
        -n, --n-jobs  the number of jobs to run
        -d, --daemon  run transfer in daemon mode
            --skip    skip hidden files
        -h, --help    show this help
        -v, --version show version information
            --debug   show more info
```

### 2.2 command line usage

```bash
# copy file in same machine
transfer -i test_data/testfile.txt -o test_data1

# push file from local machine to remote with sftp
transfer -i test_data/testfile.txt -o ssh://user:password@ip/home/zhang

# push file from local machine to remote with ftp
transfer -i test_data/testfile.txt -o ftp://user:password@ip

# pull file from remote machine to local with ftp
transfer -o test_data/ -i ftp://user:password@ip

# start remote http server
transfer --server http://0.0.0.0:8080 -i test_data

# pull file from remote machine with http
transfer -i http://127.0.0.1:8080/testfile.txt -o test_data2

# push local files to the root directory of first bucket with s3's default profile
transfer -i test_data/ -o s3

# push local files to the specific path of first bucket with s3's specific profile
transfer -i test_data/ -o s3://profile/path

# push local files to the specific path of specific bucket with s3's specific profile
transfer -i test_data/ -o s3://profile/path --bucket bucket

# pull files from the specific path of specific bucket with s3's specific profile to local
transfer -i s3://profile/path --bucket bucket -o test_data
```

> Note: transfer only support transfer files between local and aws s3, and the aws s3 credentials required to be properly configured between running transfer

> for linux users, the aws credential located at $HOME/.aws/credentials, and it should be:
```ini
[default]
aws_access_key_id = xxx
aws_secret_access_key = xxx
endpoint_url = http://host:port

[profile1]
aws_access_key_id = xxx
aws_secret_access_key = xxx
```

> more information please check the [official documents](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-files.html)