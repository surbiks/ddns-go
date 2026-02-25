package web

import (
	"encoding/json"
	"net/http"
)

// Result Result
type Result struct {
	Code int         // status
	Msg  string      // message
	Data interface{} // data
}

// returnError return error message
func returnError(w http.ResponseWriter, msg string) {
	result := &Result{}

	result.Code = http.StatusInternalServerError
	result.Msg = msg

	json.NewEncoder(w).Encode(result)
}

// returnOK	return success message
func returnOK(w http.ResponseWriter, msg string, data interface{}) {
	result := &Result{}

	result.Code = http.StatusOK
	result.Msg = msg
	result.Data = data

	json.NewEncoder(w).Encode(result)
}
