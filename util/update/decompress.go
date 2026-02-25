// Based on https://github.com/creativeprojects/go-selfupdate/blob/v1.1.1/decompress.go

package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"strings"
)

var fileTypes = map[string]func(src io.Reader, cmd string) (io.Reader, error){
	".zip":    unzip,
	".tar.gz": untar,
}

// decompressCommand 'url' parameters 'url' parameters asset URL
// file
// reader 'cmd' '.zip' '.tar.gz'
//
//   - errCannotDecompressFile
//   - errExecutableNotFoundInArchive
func decompressCommand(src io.Reader, url, cmd string) (io.Reader, error) {
	for ext, decompress := range fileTypes {
		if strings.HasSuffix(url, ext) {
			return decompress(src, cmd)
		}
	}
	log.Print("It's not a compressed file, skip decompressing")
	return src, nil
}

func unzip(src io.Reader, cmd string) (io.Reader, error) {
	// Zip file
	// HTTP response
	buf, err := io.ReadAll(src)
	if err != nil {
		return nil, fmt.Errorf("%w zip file: %v", errCannotDecompressFile, err)
	}

	r := bytes.NewReader(buf)
	z, err := zip.NewReader(r, r.Size())
	if err != nil {
		return nil, fmt.Errorf("%w zip file: %s", errCannotDecompressFile, err)
	}

	for _, file := range z.File {
		_, name := filepath.Split(file.Name)
		if !file.FileInfo().IsDir() && matchExecutableName(cmd, name) {
			return file.Open()
		}
	}

	return nil, fmt.Errorf("Executable not found in zip file: %w %q", errExecutableNotFoundInArchive, cmd)
}

func untar(src io.Reader, cmd string) (io.Reader, error) {
	gz, err := gzip.NewReader(src)
	if err != nil {
		return nil, fmt.Errorf("%w tar.gz file: %s", errCannotDecompressFile, err)
	}

	t := tar.NewReader(gz)
	for {
		h, err := t.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w tar.gz file: %s", errCannotDecompressFile, err)
		}
		_, name := filepath.Split(h.Name)
		if matchExecutableName(cmd, name) {
			return t, nil
		}
	}
	return nil, fmt.Errorf("Executable not found in tar.gz file: %w %q", errExecutableNotFoundInArchive, cmd)
}

func matchExecutableName(cmd, target string) bool {
	return cmd == target || cmd+".exe" == target
}
