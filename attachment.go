package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
)

type Attachment struct {
	name		string
	size		int64
	url			string
}

func (a *Attachment) ToString() string {
	var str strings.Builder
	
	str.WriteString(fmt.Sprintf("[%v](%v)\n", a.name, a.url))
	str.WriteString(fmt.Sprintf("Size: %d", a.size))
	return str.String()
}

// Downloads a file at a given URL.
// path: path to where the file will be downloaded.
// url: URL of the file.
func (a *Attachment) downloadFile(path string, url string) (string, error) {
	// Create the file.
	out, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer out.Close()
	
	// Download the file.
	response, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	
	// Write the downloaded data to the file.
	_, err = io.Copy(out, response.Body)
	if err != nil {
		return "", err
	}
	return path, nil
}

// Downlaods the attachment to a given directory.
// dir: File will be downloaded to this directory.
func (a *Attachment) Download(dir string) (string, error) {
	return a.downloadFile(path.Join(dir, strings.Replace(a.name, " ", "_", -1)), a.url)
}