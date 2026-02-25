package web

import (
	"encoding/json"
	"net/http"

	"github.com/jeessy2/ddns-go/v6/config"
	"github.com/jeessy2/ddns-go/v6/util"
)

func WebhookTest(writer http.ResponseWriter, request *http.Request) {
	var data struct {
		URL         string `json:"URL"`
		RequestBody string `json:"RequestBody"`
		Headers     string `json:"Headers"`
	}
	err := json.NewDecoder(request.Body).Decode(&data)
	if err != nil {
		util.Log("Data parsing failed, please refresh the page and try again")
		return
	}

	url := data.URL
	requestBody := data.RequestBody
	headers := data.Headers

	if url == "" {
		util.Log("Please enter the Webhook url")
		return
	}

	var domains = make([]*config.Domain, 1)
	domains[0] = &config.Domain{}
	domains[0].DomainName = "example.com"
	domains[0].SubDomain = "test"
	domains[0].UpdateStatus = config.UpdatedSuccess

	fakeDomains := &config.Domains{
		Ipv4Addr:    "127.0.0.1",
		Ipv4Domains: domains,
		Ipv6Addr:    "::1",
		Ipv6Domains: domains,
	}

	fakeConfig := &config.Config{
		Webhook: config.Webhook{
			WebhookURL:         url,
			WebhookRequestBody: requestBody,
			WebhookHeaders:     headers,
		},
	}

	config.ExecWebhook(fakeDomains, fakeConfig)
}
