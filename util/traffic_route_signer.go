package util

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const Version = "2018-08-01"
const Service = "DNS"
const Region = "cn-north-1"
const Host = "open.volcengineapi.com"

// sha256
func hmacSHA256(key []byte, content string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(content))
	return mac.Sum(nil)
}

// sha256 hash
func hashSHA256(content []byte) string {
	h := sha256.New()
	h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}

// request
type RequestParam struct {
	Body      []byte
	Method    string
	Date      time.Time
	Path      string
	Host      string
	QueryList url.Values
}

type Credentials struct {
	AccessKeyID     string
	SecretAccessKey string
	Service         string
	Region          string
}

// result
type SignRequest struct {
	XDate          string
	Host           string
	ContentType    string
	XContentSha256 string
	Authorization  string
}

// create DNS API request
func TrafficRouteSigner(method string, query map[string][]string, header map[string]string, ak string, sk string, action string, body []byte) (*http.Request, error) {
	// requestDNS create HTTP request
	// create HTTP request
	request, _ := http.NewRequest(method, "https://"+Host+"/", bytes.NewReader(body))
	urlVales := url.Values{}
	for k, v := range query {
		urlVales[k] = v
	}
	urlVales["Action"] = []string{action}
	urlVales["Version"] = []string{Version}
	request.URL.RawQuery = urlVales.Encode()
	for k, v := range header {
		request.Header.Set(k, v)
	}
	// create Service Region ak sk AccessKeyID SecretAccessKey handle
	//
	credential := Credentials{
		AccessKeyID:     ak,
		SecretAccessKey: sk,
		Service:         Service,
		Region:          Region,
	}
	//
	requestParam := RequestParam{
		Body:      body,
		Host:      request.Host,
		Path:      "/",
		Method:    request.Method,
		Date:      time.Now().UTC(),
		QueryList: request.URL.Query(),
	}
	// result signResult parameters
	// result
	xDate := requestParam.Date.Format("20060102T150405Z")
	shortXDate := xDate[:8]
	XContentSha256 := hashSHA256(requestParam.Body)
	contentType := "application/json"
	signResult := SignRequest{
		Host:           requestParam.Host, // Host
		XContentSha256: XContentSha256,    // body
		XDate:          xDate,             //
		ContentType:    contentType,       // Content-Type application/json
	}
	// Signature
	signedHeadersStr := strings.Join([]string{"content-type", "host", "x-content-sha256", "x-date"}, ";")
	canonicalRequestStr := strings.Join([]string{
		requestParam.Method,
		requestParam.Path,
		request.URL.RawQuery,
		strings.Join([]string{"content-type:" + contentType, "host:" + requestParam.Host, "x-content-sha256:" + XContentSha256, "x-date:" + xDate}, "\n"),
		"",
		signedHeadersStr,
		XContentSha256,
	}, "\n")
	hashedCanonicalRequest := hashSHA256([]byte(canonicalRequestStr))
	credentialScope := strings.Join([]string{shortXDate, credential.Region, credential.Service, "request"}, "/")
	stringToSign := strings.Join([]string{
		"HMAC-SHA256",
		xDate,
		credentialScope,
		hashedCanonicalRequest,
	}, "\n")
	kDate := hmacSHA256([]byte(credential.SecretAccessKey), shortXDate)
	kRegion := hmacSHA256(kDate, credential.Region)
	kService := hmacSHA256(kRegion, credential.Service)
	kSigning := hmacSHA256(kService, "request")
	signature := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))
	signResult.Authorization = fmt.Sprintf("HMAC-SHA256 Credential=%s, SignedHeaders=%s, Signature=%s", credential.AccessKeyID+"/"+credentialScope, signedHeadersStr, signature)
	// Signature HTTP Header HTTP request
	// set 5 signed HTTP headers
	request.Header.Set("Host", signResult.Host)
	request.Header.Set("Content-Type", signResult.ContentType)
	request.Header.Set("X-Date", signResult.XDate)
	request.Header.Set("X-Content-Sha256", signResult.XContentSha256)
	request.Header.Set("Authorization", signResult.Authorization)

	return request, nil
}
