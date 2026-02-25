package dns

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jeessy2/ddns-go/v6/config"
	"github.com/jeessy2/ddns-go/v6/util"
)

const zonesAPI = "https://api.cloudflare.com/client/v4/zones"

// Cloudflare Cloudflare
type Cloudflare struct {
	DNS     config.DNS
	Domains config.Domains
	TTL     int
}

// CloudflareZonesResp cloudflare zones result
type CloudflareZonesResp struct {
	CloudflareStatus
	Result []struct {
		ID     string
		Name   string
		Status string
		Paused bool
	}
}

// CloudflareRecordsResp records
type CloudflareRecordsResp struct {
	CloudflareStatus
	Result []CloudflareRecord
}

// CloudflareRecord record entity
type CloudflareRecord struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
	TTL     int    `json:"ttl"`
	Comment string `json:"comment"`
}

// CloudflareStatus status
type CloudflareStatus struct {
	Success  bool
	Messages []string
}

// Init
func (cf *Cloudflare) Init(dnsConf *config.DnsConfig, ipv4cache *util.IpCache, ipv6cache *util.IpCache) {
	cf.Domains.Ipv4Cache = ipv4cache
	cf.Domains.Ipv6Cache = ipv6cache
	cf.DNS = dnsConf.DNS
	cf.Domains.GetNewIp(dnsConf)
	if dnsConf.TTL == "" {
		// default1 auto ttl
		cf.TTL = 1
	} else {
		ttl, err := strconv.Atoi(dnsConf.TTL)
		if err != nil {
			cf.TTL = 1
		} else {
			cf.TTL = ttl
		}
	}
}

// AddUpdateDomainRecords add or update IPv4/IPv6 records
func (cf *Cloudflare) AddUpdateDomainRecords() config.Domains {
	cf.addUpdateDomainRecords("A")
	cf.addUpdateDomainRecords("AAAA")
	return cf.Domains
}

func (cf *Cloudflare) addUpdateDomainRecords(recordType string) {
	ipAddr, domains := cf.Domains.GetNewIpResult(recordType)

	if ipAddr == "" {
		return
	}

	for _, domain := range domains {
		// get zone
		result, err := cf.getZones(domain)

		if err != nil {
			util.Log("Failed to query domain info! %s", err)
			domain.UpdateStatus = config.UpdatedFailed
			return
		}

		if len(result.Result) == 0 {
			util.Log("Root domain not found in DNS provider: %s", domain.DomainName)
			domain.UpdateStatus = config.UpdatedFailed
			return
		}

		params := url.Values{}
		params.Set("type", recordType)
		// The name of DNS records in Cloudflare API expects Punycode.
		//
		// See: cloudflare/cloudflare-go#690
		params.Set("name", domain.ToASCII())
		params.Set("per_page", "50")
		// Add a comment only if it exists
		if c := domain.GetCustomParams().Get("comment"); c != "" {
			params.Set("comment", c)
		}

		zoneID := result.Result[0].ID

		var records CloudflareRecordsResp
		// getDomains update 50
		err = cf.request(
			"GET",
			fmt.Sprintf(zonesAPI+"/%s/dns_records?%s", zoneID, params.Encode()),
			nil,
			&records,
		)

		if err != nil {
			util.Log("Failed to query domain info! %s", err)
			domain.UpdateStatus = config.UpdatedFailed
			return
		}

		if !records.Success {
			util.Log("Failed to query domain info! %s", strings.Join(records.Messages, ", "))
			domain.UpdateStatus = config.UpdatedFailed
			return
		}

		if len(records.Result) > 0 {
			// update
			cf.modify(records, zoneID, domain, ipAddr)
		} else {
			// add
			cf.create(zoneID, domain, recordType, ipAddr)
		}
	}
}

// create
func (cf *Cloudflare) create(zoneID string, domain *config.Domain, recordType string, ipAddr string) {
	record := &CloudflareRecord{
		Type:    recordType,
		Name:    domain.ToASCII(),
		Content: ipAddr,
		Proxied: false,
		TTL:     cf.TTL,
		Comment: domain.GetCustomParams().Get("comment"),
	}
	record.Proxied = domain.GetCustomParams().Get("proxied") == "true"
	var status CloudflareStatus
	err := cf.request(
		"POST",
		fmt.Sprintf(zonesAPI+"/%s/dns_records", zoneID),
		record,
		&status,
	)

	if err != nil {
		util.Log("Failed to add domain %s! Result: %s", domain, err)
		domain.UpdateStatus = config.UpdatedFailed
		return
	}

	if status.Success {
		util.Log("Added domain %s successfully! IP: %s", domain, ipAddr)
		domain.UpdateStatus = config.UpdatedSuccess
	} else {
		util.Log("Failed to add domain %s! Result: %s", domain, strings.Join(status.Messages, ", "))
		domain.UpdateStatus = config.UpdatedFailed
	}
}

// modify
func (cf *Cloudflare) modify(result CloudflareRecordsResp, zoneID string, domain *config.Domain, ipAddr string) {
	for _, record := range result.Result {
		// skip if unchanged
		if record.Content == ipAddr {
			util.Log("Your's IP %s has not changed! Domain: %s", ipAddr, domain)
			continue
		}
		var status CloudflareStatus
		record.Content = ipAddr
		record.TTL = cf.TTL
		// parameters modifyproxied
		if domain.GetCustomParams().Has("proxied") {
			record.Proxied = domain.GetCustomParams().Get("proxied") == "true"
		}
		err := cf.request(
			"PUT",
			fmt.Sprintf(zonesAPI+"/%s/dns_records/%s", zoneID, record.ID),
			record,
			&status,
		)

		if err != nil {
			util.Log("Failed to updated domain %s! Result: %s", domain, err)
			domain.UpdateStatus = config.UpdatedFailed
			return
		}

		if status.Success {
			util.Log("Updated domain %s successfully! IP: %s", domain, ipAddr)
			domain.UpdateStatus = config.UpdatedSuccess
		} else {
			util.Log("Failed to updated domain %s! Result: %s", domain, strings.Join(status.Messages, ", "))
			domain.UpdateStatus = config.UpdatedFailed
		}
	}
}

// get domain record list
func (cf *Cloudflare) getZones(domain *config.Domain) (result CloudflareZonesResp, err error) {
	params := url.Values{}
	params.Set("name", domain.DomainName)
	params.Set("status", "active")
	params.Set("per_page", "50")

	err = cf.request(
		"GET",
		fmt.Sprintf(zonesAPI+"?%s", params.Encode()),
		nil,
		&result,
	)

	return
}

// request shared request method
func (cf *Cloudflare) request(method string, url string, data interface{}, result interface{}) (err error) {
	jsonStr := make([]byte, 0)
	if data != nil {
		jsonStr, _ = json.Marshal(data)
	}
	req, err := http.NewRequest(
		method,
		url,
		bytes.NewBuffer(jsonStr),
	)
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+cf.DNS.Secret)
	req.Header.Set("Content-Type", "application/json")

	client := util.CreateHTTPClient()
	resp, err := client.Do(req)
	err = util.GetHTTPResponse(resp, err, result)

	return
}
