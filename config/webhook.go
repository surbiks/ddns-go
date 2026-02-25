package config

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/jeessy2/ddns-go/v6/util"
)

// Webhook Webhook
type Webhook struct {
	WebhookURL         string
	WebhookRequestBody string
	WebhookHeaders     string
}

// updateStatusType updatestatus
type updateStatusType string

const (
	// UpdatedNothing no changed
	UpdatedNothing updateStatusType = "no changed"
	// UpdatedFailed updatefailed
	UpdatedFailed = "failed"
	// UpdatedSuccess updatesuccess
	UpdatedSuccess = "success"
)

// updatefailed
var updatedFailedTimes = 0

// hasJSONPrefix returns true if the string starts with a JSON open brace.
func hasJSONPrefix(s string) bool {
	return strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[")
}

// ExecWebhook add or update IPv4/IPv6 records, updatefailed
func ExecWebhook(domains *Domains, conf *Config) (v4Status updateStatusType, v6Status updateStatusType) {
	v4Status = getDomainsStatus(domains.Ipv4Domains)
	v6Status = getDomainsStatus(domains.Ipv6Domains)

	if conf.WebhookURL != "" && (v4Status != UpdatedNothing || v6Status != UpdatedNothing) {
		// 3 failed webhook
		if v4Status == UpdatedFailed || v6Status == UpdatedFailed {
			updatedFailedTimes++
			if updatedFailedTimes != 3 {
				util.Log("Webhook will not be triggered, only trigger once when the third failure, current failure times: %d", updatedFailedTimes)
				return
			}
		} else {
			updatedFailedTimes = 0
		}

		// success failed webhook
		method := "GET"
		postPara := ""
		contentType := "application/x-www-form-urlencoded"
		if conf.WebhookRequestBody != "" {
			method = "POST"
			postPara = replacePara(domains, conf.WebhookRequestBody, v4Status, v6Status)
			if json.Valid([]byte(postPara)) {
				contentType = "application/json"
			} else if hasJSONPrefix(postPara) {
				// RequestBody JSON JSON
				util.Log("Webhook RequestBody JSON is invalid")
			}
		}
		requestURL := replacePara(domains, conf.WebhookURL, v4Status, v6Status)
		u, err := url.Parse(requestURL)
		if err != nil {
			util.Log("Webhook url is incorrect")
			return
		}

		q, _ := url.ParseQuery(u.RawQuery)
		u.RawQuery = q.Encode()

		req, err := http.NewRequest(method, u.String(), strings.NewReader(postPara))
		if err != nil {
			util.Log("Failed to call Webhook! Exception: %s", err)
			return
		}

		headers := extractHeaders(conf.WebhookHeaders)
		for key, value := range headers {
			req.Header.Add(key, value)
		}
		req.Header.Add("content-type", contentType)

		clt := util.CreateHTTPClient()
		resp, err := clt.Do(req)
		body, err := util.GetHTTPResponseOrg(resp, err)
		if err == nil {
			util.Log("Successfully called Webhook! Response body: %s", string(body))
		} else {
			util.Log("Failed to call Webhook! Exception: %s", err)
		}
	}
	return
}

// getDomainsStatus get domain status
func getDomainsStatus(domains []*Domain) updateStatusType {
	successNum := 0
	for _, v46 := range domains {
		switch v46.UpdateStatus {
		case UpdatedFailed:
			// failed failed
			return UpdatedFailed
		case UpdatedSuccess:
			successNum++
		}
	}

	if successNum > 0 {
		// success success
		return UpdatedSuccess
	}
	return UpdatedNothing
}

// replacePara parameters
func replacePara(domains *Domains, orgPara string, ipv4Result updateStatusType, ipv6Result updateStatusType) string {
	return strings.NewReplacer(
		"#{ipv4Addr}", domains.Ipv4Addr,
		"#{ipv4Result}", util.LogStr(string(ipv4Result)), // i18n
		"#{ipv4Domains}", getDomainsStr(domains.Ipv4Domains),
		"#{ipv6Addr}", domains.Ipv6Addr,
		"#{ipv6Result}", util.LogStr(string(ipv6Result)), // i18n
		"#{ipv6Domains}", getDomainsStr(domains.Ipv6Domains),
	).Replace(orgPara)
}

// getDomainsStr domain
func getDomainsStr(domains []*Domain) string {
	str := ""
	for i, v46 := range domains {
		str += v46.String()
		if i != len(domains)-1 {
			str += ","
		}
	}

	return str
}

// extractHeaders converts s into a map of headers.
//
// See also: https://github.com/appleboy/gorush/blob/v1.17.0/notify/feedback.go#L15
func extractHeaders(s string) map[string]string {
	lines := util.SplitLines(s)
	headers := make(map[string]string, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			util.Log("Webhook header is invalid: %s", line)
			continue
		}

		k, v := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		headers[k] = v
	}

	return headers
}
