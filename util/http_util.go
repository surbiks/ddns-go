package util

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// GetHTTPResponse handleHTTPresult json
func GetHTTPResponse(resp *http.Response, err error, result interface{}) error {
	body, err := GetHTTPResponseOrg(resp, err)

	if err == nil {
		// log.Println(string(body))
		if len(body) != 0 {
			err = json.Unmarshal(body, &result)
		}
	}

	return err

}

// GetHTTPResponseOrg handleHTTPresult byte
func GetHTTPResponseOrg(resp *http.Response, err error) ([]byte, error) {
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	lr := io.LimitReader(resp.Body, 1024000)
	body, err := io.ReadAll(lr)

	if err != nil {
		return nil, err
	}

	// 300 status code
	if resp.StatusCode >= 300 {
		err = fmt.Errorf("%s", LogStr("Response body: %s ,Response status code: %d", string(body), resp.StatusCode))
	}

	return body, err
}
