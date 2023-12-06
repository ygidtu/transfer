package client

import (
	"fmt"
	"github.com/k0kubun/go-ansi"
	"github.com/schollz/progressbar/v3"
	"github.com/ygidtu/transfer/base"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// bytesBar 自定义的progress风格，用以展示文件的读写进度
func bytesBar(size int64, name string) *progressbar.ProgressBar {
	if len(name) > 50 {
		name = fmt.Sprintf("%s...", name[0:51])
	}

	return progressbar.NewOptions(int(size),
		progressbar.OptionSetWriter(ansi.NewAnsiStderr()), // 使用ansi防止奇怪的换行等显示错误
		progressbar.OptionUseANSICodes(true),              // avoid progressbar downsize error
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetDescription(name),
		progressbar.OptionFullWidth(),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionSetRenderBlankState(true),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() {
			fmt.Fprint(os.Stderr, "\n")
		}),
		progressbar.OptionSetWidth(10),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}))
}

// Transfer 文件对文件的传输对象
type Transfer struct {
	source     *File                    // 输入的源文件或源目录地址，不指代目录下的其他文件
	target     *File                    // 目标文件
	bar        *progressbar.ProgressBar // 传输进度条
	concurrent int                      // 并行数量
}

// Describe 更改进度条指示文本
func (transfer *Transfer) Describe(msg string) {
	//transfer.bar.Clear()
	transfer.bar.Describe(msg)
}

/*
GetTarget 根据源文件地址生成目标文件地址
@src: 源文件地址
*/
func (transfer *Transfer) GetTarget(src *File) *File {
	return src.GetTarget(transfer.source, transfer.target)
}

/*
validate 检查要传输的两个源文件和目标文件的MD5是否对应
@src: 要传输的源文件，若transfer.source为文件，则src为该文件；否则为该目录下的子文件
@dst: 对应的目标文件
*/
func (transfer *Transfer) validate(src *File, dst *File) (bool, error) {
	if err := src.GetMd5(); err != nil {
		return false, err
	}

	if err := dst.GetMd5(); err != nil {
		return false, err
	}

	return src.Md5 == dst.Md5, nil
}

/*
Transfer 开始传输文件
@src: 要传输的源文件，若transfer.source为文件，则src为该文件；否则为该目录下的子文件
@dst: 对应的目标文件
*/
func (transfer *Transfer) Transfer(src *File, dst *File) error {
	log.Debugf("transfer %v -> %v", src.Path, dst.Path)
	valid, err := transfer.validate(src, dst)

	if !valid {
		if dst.Source() == Aws && src.Source() != Local || src.Source() == Aws && dst.Source() != Local {
			log.Fatalf("the aws only can transfer data with local, not %v %v", src.Source(), dst.Source())
		}

		if !dst.Exists() && filepath.Dir(dst.Path) != "/" {
			err = dst.MkParent()
			if err != nil {
				return fmt.Errorf("failed to create directory for %v", dst.Path)
			}
		}

		trunc := src.Size < dst.Size
		resumeFrom := int64(0)

		if src.Size > dst.Size && dst.Size > 0 {
			log.Warnf("resume file from %d", dst.Size)
			resumeFrom = dst.Size
			_ = transfer.bar.Add(int(dst.Size))
		}

		if dst.Source() == Aws {
			r, err := src.ReadSeeker()
			if err != nil {
				return err
			}
			defer r.Close()

			if err := dst.Write(r); err != nil {
				return err
			}
			transfer.bar.Add(int(src.Size))
		} else {
			r, err := src.Reader(resumeFrom)
			if err != nil {
				return err
			}
			defer r.Close()

			reader := progressbar.NewReader(r, transfer.bar)
			if err := dst.WriteAt(&reader, trunc); err != nil {
				return err
			}
		}

	} else {
		transfer.bar.Add(int(src.Size))
	}
	return nil
}

// Start 启动传输过程
func (transfer *Transfer) Start() {
	if transfer.target == nil {
		if err := transfer.source.client.(*HttpClient).startServer(); err != nil {
			log.Fatal(err)
		}
		return
	}

	var wg sync.WaitGroup

	taskChan := make(chan *File)
	for i := 0; i < transfer.concurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				f, ok := <-taskChan

				if !ok {
					break
				}

				transfer.Describe(f.ID)
				if err := transfer.Transfer(f, transfer.GetTarget(f)); err != nil {
					base.SugaredLog.Warn(err)
				}
			}
		}()
	}

	log.Debugf("source = %v", transfer.source)
	files, err := transfer.source.Children()
	if err != nil {
		base.SugaredLog.Fatal(err)
	}
	transfer.bar = bytesBar(files.Total, "transfer")
	log.Debugf("prepare to transfer %d files", len(files.Files))
	for i, f := range files.Files {
		f.ID = fmt.Sprintf("[%d/%d] %s", i+1, len(files.Files), f.ShortID())
		taskChan <- f
	}
	close(taskChan)
	wg.Wait()
	_ = transfer.bar.Close()
	fmt.Println()
}
