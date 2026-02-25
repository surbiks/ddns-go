package dns

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/jeessy2/ddns-go/v6/config"
	"github.com/jeessy2/ddns-go/v6/util"
)

type Callback struct {
	DNS      config.DNS
	Domains  config.Domains
	TTL      string
	lastIpv4 string
	lastIpv6 string
}

// Init
func (cb *Callback) Init(dnsConf *config.DnsConfig, ipv4cache *util.IpCache, ipv6cache *util.IpCache) {
	cb.Domains.Ipv4Cache = ipv4cache
	cb.Domains.Ipv6Cache = ipv6cache
	cb.lastIpv4 = ipv4cache.Addr
	cb.lastIpv6 = ipv6cache.Addr

	cb.DNS = dnsConf.DNS
	cb.Domains.GetNewIp(dnsConf)
	if dnsConf.TTL == "" {
		// default600
		cb.TTL = "600"
	} else {
		cb.TTL = dnsConf.TTL
	}
}

// AddUpdateDomainRecords add or update IPv4/IPv6 records
func (cb *Callback) AddUpdateDomainRecords() config.Domains {
	cb.addUpdateDomainRecords("A")
	cb.addUpdateDomainRecords("AAAA")
	return cb.Domains
}

func (cb *Callback) addUpdateDomainRecords(recordType string) {
	ipAddr, domains := cb.Domains.GetNewIpResult(recordType)

	if ipAddr == "" {
		return
	}

	// Webhooknotification
	if recordType == "A" {
		if cb.lastIpv4 == ipAddr {
			util.Log("Your's IPv4 has not changed, %s request has not been triggered", "Callback")
			return
		}
	} else {
		if cb.lastIpv6 == ipAddr {
			util.Log("Your's IPv6 has not changed, %s request has not been triggered", "Callback")
			return
		}
	}

	for _, domain := range domains {
		method := "GET"
		postPara := ""
		contentType := "application/x-www-form-urlencoded"
		if cb.DNS.Secret != "" {
			method = "POST"
			postPara = replacePara(cb.DNS.Secret, ipAddr, domain, recordType, cb.TTL)
			if json.Valid([]byte(postPara)) {
				contentType = "application/json"
			}
		}
		requestURL := replacePara(cb.DNS.ID, ipAddr, domain, recordType, cb.TTL)
		u, err := url.Parse(requestURL)
		if err != nil {
			util.Log("Callback url is incorrect")
			return
		}
		req, err := http.NewRequest(method, u.String(), strings.NewReader(postPara))
		if err != nil {
			util.Log("Exception: %s", err)
			domain.UpdateStatus = config.UpdatedFailed
			return
		}
		req.Header.Add("content-type", contentType)

		clt := util.CreateHTTPClient()
		resp, err := clt.Do(req)
		body, err := util.GetHTTPResponseOrg(resp, err)
		if err == nil {
			util.Log("Successfully called Callback! Domain: %s, IP: %s, Response body: %s", domain, ipAddr, string(body))
			domain.UpdateStatus = config.UpdatedSuccess
		} else {
			util.Log("Callback call failed, Exception: %s", err)
			domain.UpdateStatus = config.UpdatedFailed
		}
	}
}

// replacePara parameters
func replacePara(orgPara, ipAddr string, domain *config.Domain, recordType string, ttl string) string {
	// params map add parameters
	params := map[string]string{
		"ip":         ipAddr,
		"domain":     domain.String(),
		"recordType": recordType,
		"ttl":        ttl,
	}

	// domain parameters
	for k, v := range domain.GetCustomParams() {
		if len(v) == 1 {
			params[k] = v[0]
		}
	}

	// map [NewReplacer] parameters
	// map 2 kv 2
	oldnew := make([]string, 0, len(params)*2)
	for k, v := range params {
		k = fmt.Sprintf("#{%s}", k)
		oldnew = append(oldnew, k, v)
	}

	return strings.NewReplacer(oldnew...).Replace(orgPara)
}
