package dns

import (
	"bytes"
	"net/http"
	"net/url"

	"github.com/jeessy2/ddns-go/v6/config"
	"github.com/jeessy2/ddns-go/v6/util"
)

const (
	alidnsEndpoint string = "https://alidns.aliyuncs.com/"
)

// https://help.aliyun.com/document_detail/29776.html?spm=a2c4g.11186623.6.672.715a45caji9dMA
// Alidns Alidns
type Alidns struct {
	DNS     config.DNS
	Domains config.Domains
	TTL     string
}

// AlidnsRecord record
type AlidnsRecord struct {
	DomainName string
	RecordID   string
	Value      string
}

// AlidnsSubDomainRecords record
type AlidnsSubDomainRecords struct {
	TotalCount    int
	DomainRecords struct {
		Record []AlidnsRecord
	}
}

// AlidnsResp modify/add result
type AlidnsResp struct {
	RecordID  string
	RequestID string
}

// Init
func (ali *Alidns) Init(dnsConf *config.DnsConfig, ipv4cache *util.IpCache, ipv6cache *util.IpCache) {
	ali.Domains.Ipv4Cache = ipv4cache
	ali.Domains.Ipv6Cache = ipv6cache
	ali.DNS = dnsConf.DNS
	ali.Domains.GetNewIp(dnsConf)
	if dnsConf.TTL == "" {
		// default600s
		ali.TTL = "600"
	} else {
		ali.TTL = dnsConf.TTL
	}
}

// AddUpdateDomainRecords add or update IPv4/IPv6 records
func (ali *Alidns) AddUpdateDomainRecords() config.Domains {
	ali.addUpdateDomainRecords("A")
	ali.addUpdateDomainRecords("AAAA")
	return ali.Domains
}

func (ali *Alidns) addUpdateDomainRecords(recordType string) {
	ipAddr, domains := ali.Domains.GetNewIpResult(recordType)

	if ipAddr == "" {
		return
	}

	for _, domain := range domains {
		var records AlidnsSubDomainRecords
		// get current domain info
		params := domain.GetCustomParams()
		params.Set("Action", "DescribeSubDomainRecords")
		params.Set("DomainName", domain.DomainName)
		params.Set("SubDomain", domain.GetFullDomain())
		params.Set("Type", recordType)
		err := ali.request(params, &records)

		if err != nil {
			util.Log("Failed to query domain info! %s", err)
			domain.UpdateStatus = config.UpdatedFailed
			return
		}

		if records.TotalCount > 0 {
			// first by default
			recordSelected := records.DomainRecords.Record[0]
			if params.Has("RecordId") {
				for i := 0; i < len(records.DomainRecords.Record); i++ {
					if records.DomainRecords.Record[i].RecordID == params.Get("RecordId") {
						recordSelected = records.DomainRecords.Record[i]
					}
				}
			}
			// update
			ali.modify(recordSelected, domain, recordType, ipAddr)
		} else {
			// create
			ali.create(domain, recordType, ipAddr)
		}

	}
}

// create
func (ali *Alidns) create(domain *config.Domain, recordType string, ipAddr string) {
	params := domain.GetCustomParams()
	params.Set("Action", "AddDomainRecord")
	params.Set("DomainName", domain.DomainName)
	params.Set("RR", domain.GetSubDomain())
	params.Set("Type", recordType)
	params.Set("Value", ipAddr)
	params.Set("TTL", ali.TTL)

	var result AlidnsResp
	err := ali.request(params, &result)

	if err != nil {
		util.Log("Failed to add domain %s! Result: %s", domain, err)
		domain.UpdateStatus = config.UpdatedFailed
		return
	}

	if result.RecordID != "" {
		util.Log("Added domain %s successfully! IP: %s", domain, ipAddr)
		domain.UpdateStatus = config.UpdatedSuccess
	} else {
		util.Log("Failed to add domain %s! Result: %s", domain, "returned empty RecordId")
		domain.UpdateStatus = config.UpdatedFailed
	}
}

// modify
func (ali *Alidns) modify(recordSelected AlidnsRecord, domain *config.Domain, recordType string, ipAddr string) {

	// skip if unchanged
	if recordSelected.Value == ipAddr {
		util.Log("Your's IP %s has not changed! Domain: %s", ipAddr, domain)
		return
	}

	params := domain.GetCustomParams()
	params.Set("Action", "UpdateDomainRecord")
	params.Set("RR", domain.GetSubDomain())
	params.Set("RecordId", recordSelected.RecordID)
	params.Set("Type", recordType)
	params.Set("Value", ipAddr)
	params.Set("TTL", ali.TTL)

	var result AlidnsResp
	err := ali.request(params, &result)

	if err != nil {
		util.Log("Failed to updated domain %s! Result: %s", domain, err)
		domain.UpdateStatus = config.UpdatedFailed
		return
	}

	if result.RecordID != "" {
		util.Log("Updated domain %s successfully! IP: %s", domain, ipAddr)
		domain.UpdateStatus = config.UpdatedSuccess
	} else {
		util.Log("Failed to updated domain %s! Result: %s", domain, "returned empty RecordId")
		domain.UpdateStatus = config.UpdatedFailed
	}
}

// request shared request method
func (ali *Alidns) request(params url.Values, result interface{}) (err error) {
	method := http.MethodGet
	util.AliyunSigner(ali.DNS.ID, ali.DNS.Secret, &params, method, "2015-01-09")

	req, err := http.NewRequest(
		method,
		alidnsEndpoint,
		bytes.NewBuffer(nil),
	)
	req.URL.RawQuery = params.Encode()

	if err != nil {
		return
	}

	client := util.CreateHTTPClient()
	resp, err := client.Do(req)
	err = util.GetHTTPResponse(resp, err, result)

	return
}
