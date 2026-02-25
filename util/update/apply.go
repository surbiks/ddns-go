// Based on https://github.com/inconshreveable/go-update/blob/7a872911e5b39953310f0a04161f0d50c7e63755/apply.go

package update

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// apply io.Reader update targetPath file
//
// apply update
//
// 1. create file /path/to/target.new updatefile
//
// 2. /path/to/target /path/to/target.old
//
// 3. /path/to/target.new /path/to/target
//
// 4. success delete /path/to/target.old
//
// 5. failed /path/to/target.old
// /path/to/target
//
// failed file status 4 5
// file file
// notification message
func apply(update io.Reader, targetPath string) error {
	newBytes, err := io.ReadAll(update)
	if err != nil {
		return err
	}

	// get file
	updateDir := filepath.Dir(targetPath)
	filename := filepath.Base(targetPath)

	// file
	newPath := filepath.Join(updateDir, fmt.Sprintf("%s.new", filename))
	fp, err := os.OpenFile(newPath, os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer fp.Close()

	_, err = io.Copy(fp, bytes.NewReader(newBytes))
	if err != nil {
		return err
	}

	// fp.Close() Windows file
	// file "in use" status
	fp.Close()

	// file update file
	oldPath := filepath.Join(updateDir, fmt.Sprintf("%s.old", filename))

	// delete file - Windows
	// 1. successupdate Windows delete .old file
	// 2. file Windows failed
	_ = os.Remove(oldPath)

	// file file
	err = os.Rename(targetPath, oldPath)
	if err != nil {
		return err
	}

	// file
	err = os.Rename(newPath, targetPath)

	if err != nil {
		// failed
		//
		// file status success file
		// file file file
		// file path
		rerr := os.Rename(oldPath, targetPath)
		if rerr != nil {
			return err
		}

		return err
	}

	// success delete file
	err = os.Remove(oldPath)
	if err != nil {
		if runtime.GOOS == "windows" {
			// Windows delete .old file delete "Access is denied"
			// start delete file
			//
			//
			// https://stackoverflow.com/a/73585620
			exec.Command("cmd.exe", "/c", "ping 127.0.0.1 -n 2 > NUL & del "+oldPath).Start()
			return nil
		}

		return err
	}

	return nil
}
