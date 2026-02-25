package web

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
)

// MemoryLogs logs in memory
type MemoryLogs struct {
	MaxNum int      // max saved entries
	Logs   []string //
}

func (mlogs *MemoryLogs) Write(p []byte) (n int, err error) {
	mlogs.Logs = append(mlogs.Logs, string(p))
	// handle log count
	if len(mlogs.Logs) > mlogs.MaxNum {
		mlogs.Logs = mlogs.Logs[len(mlogs.Logs)-mlogs.MaxNum:]
	}
	return len(p), nil
}

var mlogs = &MemoryLogs{MaxNum: 50}

// initialize logs
func init() {
	log.SetOutput(io.MultiWriter(mlogs, os.Stdout))
	// log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

// Logs web
func Logs(writer http.ResponseWriter, request *http.Request) {
	// mlogs.Logs json
	logs, _ := json.Marshal(mlogs.Logs)
	writer.Write(logs)
}

// ClearLog
func ClearLog(writer http.ResponseWriter, request *http.Request) {
	mlogs.Logs = mlogs.Logs[:0]
}
