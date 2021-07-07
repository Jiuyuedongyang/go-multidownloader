package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
)

type Downloader struct {
	concurrency int
}

func NewDownloader(concurrency int) *Downloader {
	return &Downloader{concurrency: concurrency}
}

func (d *Downloader) Download(strURL, filename string) error {
	if filename == "" {
		filename = path.Base(strURL)
	}

	resp, err := http.Head(strURL)
	if err != nil {
		fmt.Println("http.Head(strURL) err :", err)
		return err
	}
	if resp.StatusCode == 200 && resp.Header.Get("Accept-Ranges") == "bytes" {
		return d.multiDownload(strURL, filename, int(resp.ContentLength))
	}
	return d.singleDownload(strURL, filename)

}

func (d *Downloader) multiDownload(strURL, filename string, contentLen int) error {
	partSize := contentLen / d.concurrency

	//创建部分文件的存放目录
	partDir := d.getPartDir(filename)
	os.Mkdir(partDir, 0777)
	defer os.RemoveAll(partDir)
	var wg sync.WaitGroup
	wg.Add(d.concurrency)
	rangeStart := 0
	for i := 0; i < d.concurrency; i++ {
		//并发请求
		go func(i, rangeStart int) {
			defer wg.Done()
			rangeEnd := rangeStart + partSize
			//最后一部分，总长度不能超过ContentLength
			if i == d.concurrency-1 {
				rangeEnd = contentLen
			}
			d.downloadPartial(strURL, filename, rangeStart, rangeEnd, i)
			rangeStart += partSize + 1

		}(i, rangeStart)
		wg.Wait()
		d.merge(filename)
	}

	return nil
}

func (d *Downloader) merge(filename string) error {
	distFile, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer distFile.Close()
	for i := 0; i < d.concurrency; i++ {
		partFileName := d.getPartFilename(filename, i)
		partFile, err := os.Open(partFileName)
		if err != nil {
			return err
		}
		io.Copy(distFile, partFile)
		partFile.Close()
		os.Remove(partFileName)
	}

	return nil

}
func (d *Downloader) singleDownload(strURL, filename string) error {
	return nil
}

func (d *Downloader) downloadPartial(strURL, filename string, rangeStart, rangeEnd, i int) {
	if rangeStart >= rangeEnd {
		fmt.Println("rangeStart>=rangeEnd 无效，请重新输入")
		return
	}
	req, err := http.NewRequest("GET", strURL, nil)
	if err != nil {
		log.Fatalln("http.NewRequest error : ", err)
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", rangeStart, rangeEnd))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalln("http.DefaultClient.Do(req) err : ", err)
	}
	defer resp.Body.Close()

	flags := os.O_CREATE | os.O_WRONLY
	partFile, err := os.OpenFile(d.getPartFilename(filename, i), flags, 0666)
	if err != nil {
		log.Fatal("os.OpenFile(d.getPartFilename(filename,i),flags,0666) err : ", err)
	}
	defer partFile.Close()
	buf := make([]byte, 32*1024)
	_, err = io.CopyBuffer(partFile, resp.Body, buf)
	if err != nil {
		if err == io.EOF {
			return
		}
		log.Fatal("io.CopyBuffer(partFile,resp.Body,buf) err : ", err)
	}
}

// getPartDir 部分文件存放的目录
func (d *Downloader) getPartDir(filename string) string {
	return strings.SplitN(filename, ",", 2)[0]

}

// getPartFilename 构造部分文件的名字
func (d *Downloader) getPartFilename(filename string, partNum int) string {
	partDir := d.getPartDir(filename)
	return fmt.Sprintf("%s/%s-%d", partDir, filename, partNum)

}
