package dns

import (
	"net/url"

	"github.com/jeessy2/ddns-go/v6/config"
	"github.com/jeessy2/ddns-go/v6/util"
)

const (
	recordListAPI   string = "https://dnsapi.cn/Record.List"
	recordModifyURL string = "https://dnsapi.cn/Record.Modify"
	recordCreateAPI string = "https://dnsapi.cn/Record.Create"
)

// https://cloud.tencent.com/document/api/302/8516
// Dnspod dns
type Dnspod struct {
	DNS     config.DNS
	Domains config.Domains
	TTL     string
}

// DnspodRecord DnspodRecord
type DnspodRecord struct {
	ID      string
	Name    string
	Type    string
	Value   string
	Enabled string
}

// DnspodRecordListResp recordListAPIresult
type DnspodRecordListResp struct {
	DnspodStatus
	Records []DnspodRecord
}

// DnspodStatus DnspodStatus
type DnspodStatus struct {
	Status struct {
		Code    string
		Message string
	}
}

// Init
func (dnspod *Dnspod) Init(dnsConf *config.DnsConfig, ipv4cache *util.IpCache, ipv6cache *util.IpCache) {
	dnspod.Domains.Ipv4Cache = ipv4cache
	dnspod.Domains.Ipv6Cache = ipv6cache
	dnspod.DNS = dnsConf.DNS
	dnspod.Domains.GetNewIp(dnsConf)
	if dnsConf.TTL == "" {
		// default600s
		dnspod.TTL = "600"
	} else {
		dnspod.TTL = dnsConf.TTL
	}
}

// AddUpdateDomainRecords add or update IPv4/IPv6 records
func (dnspod *Dnspod) AddUpdateDomainRecords() config.Domains {
	dnspod.addUpdateDomainRecords("A")
	dnspod.addUpdateDomainRecords("AAAA")
	return dnspod.Domains
}

func (dnspod *Dnspod) addUpdateDomainRecords(recordType string) {
	ipAddr, domains := dnspod.Domains.GetNewIpResult(recordType)

	if ipAddr == "" {
		return
	}

	for _, domain := range domains {
		result, err := dnspod.getRecordList(domain, recordType)
		if err != nil {
			util.Log("Failed to query domain info! %s", err)
			domain.UpdateStatus = config.UpdatedFailed
			return
		}

		if len(result.Records) > 0 {
			// first by default
			recordSelected := result.Records[0]
			params := domain.GetCustomParams()
			if params.Has("record_id") {
				for i := 0; i < len(result.Records); i++ {
					if result.Records[i].ID == params.Get("record_id") {
						recordSelected = result.Records[i]
					}
				}
			}
			// update
			dnspod.modify(recordSelected, domain, recordType, ipAddr)
		} else {
			// add
			dnspod.create(domain, recordType, ipAddr)
		}
	}
}

// create
func (dnspod *Dnspod) create(domain *config.Domain, recordType string, ipAddr string) {
	params := domain.GetCustomParams()
	params.Set("login_token", dnspod.DNS.ID+","+dnspod.DNS.Secret)
	params.Set("domain", domain.DomainName)
	params.Set("sub_domain", domain.GetSubDomain())
	params.Set("record_type", recordType)
	params.Set("value", ipAddr)
	params.Set("ttl", dnspod.TTL)
	params.Set("format", "json")

	if !params.Has("record_line") {
		params.Set("record_line", "\u9ed8\u8ba4")
	}

	status, err := dnspod.request(recordCreateAPI, params)

	if err != nil {
		util.Log("Failed to add domain %s! Result: %s", domain, err)
		domain.UpdateStatus = config.UpdatedFailed
		return
	}

	if status.Status.Code == "1" {
		util.Log("Added domain %s successfully! IP: %s", domain, ipAddr)
		domain.UpdateStatus = config.UpdatedSuccess
	} else {
		util.Log("Failed to add domain %s! Result: %s", domain, status.Status.Message)
		domain.UpdateStatus = config.UpdatedFailed
	}
}

// modify
func (dnspod *Dnspod) modify(record DnspodRecord, domain *config.Domain, recordType string, ipAddr string) {

	// skip if unchanged
	if record.Value == ipAddr {
		util.Log("Your's IP %s has not changed! Domain: %s", ipAddr, domain)
		return
	}

	params := domain.GetCustomParams()
	params.Set("login_token", dnspod.DNS.ID+","+dnspod.DNS.Secret)
	params.Set("domain", domain.DomainName)
	params.Set("sub_domain", domain.GetSubDomain())
	params.Set("record_type", recordType)
	params.Set("value", ipAddr)
	params.Set("ttl", dnspod.TTL)
	params.Set("format", "json")
	params.Set("record_id", record.ID)

	if !params.Has("record_line") {
		params.Set("record_line", "\u9ed8\u8ba4")
	}

	status, err := dnspod.request(recordModifyURL, params)

	if err != nil {
		util.Log("Failed to updated domain %s! Result: %s", domain, err)
		domain.UpdateStatus = config.UpdatedFailed
		return
	}

	if status.Status.Code == "1" {
		util.Log("Updated domain %s successfully! IP: %s", domain, ipAddr)
		domain.UpdateStatus = config.UpdatedSuccess
	} else {
		util.Log("Failed to updated domain %s! Result: %s", domain, status.Status.Message)
		domain.UpdateStatus = config.UpdatedFailed
	}
}

// request sends a POST request to the given API with the given values.
func (dnspod *Dnspod) request(apiAddr string, values url.Values) (status DnspodStatus, err error) {
	client := util.CreateHTTPClient()
	resp, err := client.PostForm(
		apiAddr,
		values,
	)

	err = util.GetHTTPResponse(resp, err, &status)

	return
}

// get domain record list
func (dnspod *Dnspod) getRecordList(domain *config.Domain, typ string) (result DnspodRecordListResp, err error) {

	params := domain.GetCustomParams()
	params.Set("login_token", dnspod.DNS.ID+","+dnspod.DNS.Secret)
	params.Set("domain", domain.DomainName)
	params.Set("record_type", typ)
	params.Set("sub_domain", domain.GetSubDomain())
	params.Set("format", "json")

	client := util.CreateHTTPClient()
	resp, err := client.PostForm(
		recordListAPI,
		params,
	)

	err = util.GetHTTPResponse(resp, err, &result)

	return
}
