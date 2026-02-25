package dns

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/jeessy2/ddns-go/v6/config"
	"github.com/jeessy2/ddns-go/v6/util"
	"golang.org/x/net/idna"
)

// https://cloud.tencent.com/document/api/1552/80730
const (
	edgeoneEndPoint = "https://teo.tencentcloudapi.com"
	edgeoneVersion  = "2022-09-01"
)

type EdgeOne struct {
	DNS     config.DNS
	Domains config.Domains
	TTL     int
}

type EdgeOneRecord struct {
	ZoneId   string `json:"ZoneId"`
	Name     string `json:"Name"` // FullDomain
	Type     string `json:"Type"` // record type, e.g. A AAAA
	Content  string `json:"Content"`
	Location string `json:"Location"`
	TTL      int    `json:"TTL"`
	Weight   int    `json:"Weight,omitempty"`
	RecordId string `json:"RecordId,omitempty"`
	Status   string `json:"Status,omitempty"`
}

type EdgeOneRecordResponse struct {
	EdgeOneStatus
	Response struct {
		DnsRecords []EdgeOneRecord `json:"DnsRecords"`
		TotalCount int             `json:"TotalCount"`
	}
}

type EdgeOneZoneResponse struct {
	EdgeOneStatus
	Response struct {
		TotalCount int `json:"TotalCount"`
		Zones      []struct {
			ZoneId   string `json:"ZoneId"`
			ZoneName string `json:"ZoneName"`
		} `json:"Zones"`
	}
}

type Filter struct {
	Name   string   `json:"Name"`
	Values []string `json:"Values"`
}

type EdgeOneDescribeDns struct {
	ZoneId  string   `json:"ZoneId,omitempty"`
	Filters []Filter `json:"Filters"`
}

// https://cloud.tencent.com/document/product/1552/80729
type EdgeOneStatus struct {
	Response struct {
		Error struct {
			Code    string
			Message string
		}
	}
}

// Init
func (eo *EdgeOne) Init(dnsConf *config.DnsConfig, ipv4cache *util.IpCache, ipv6cache *util.IpCache) {
	eo.Domains.Ipv4Cache = ipv4cache
	eo.Domains.Ipv6Cache = ipv6cache
	eo.DNS = dnsConf.DNS
	eo.Domains.GetNewIp(dnsConf)
	if dnsConf.TTL == "" {
		// default 600s
		eo.TTL = 600
	} else {
		ttl, err := strconv.Atoi(dnsConf.TTL)
		if err != nil {
			eo.TTL = 600
		} else {
			eo.TTL = ttl
		}
	}
}

// AddUpdateDomainRecords add or update IPv4/IPv6 records
func (eo *EdgeOne) AddUpdateDomainRecords() config.Domains {
	eo.addUpdateDomainRecords("A")
	eo.addUpdateDomainRecords("AAAA")
	return eo.Domains
}

func (eo *EdgeOne) addUpdateDomainRecords(recordType string) {
	ipAddr, domains := eo.Domains.GetNewIpResult(recordType)

	if ipAddr == "" {
		return
	}

	for _, domain := range domains {
		zoneResult, err := eo.getZone(domain.DomainName)
		if err != nil || zoneResult.Response.TotalCount <= 0 || zoneResult.Response.Zones[0].ZoneName != domain.DomainName {
			util.Log("Failed to query domain info! %s", err)
			domain.UpdateStatus = config.UpdatedFailed
			return
		}
		zoneId := zoneResult.Response.Zones[0].ZoneId
		recordResult, err := eo.getRecordList(domain, recordType, zoneId)
		if err != nil {
			util.Log("Failed to query domain info! %s", err)
			domain.UpdateStatus = config.UpdatedFailed
			return
		}

		params := domain.GetCustomParams()
		var isValid func(*EdgeOneRecord) bool
		if params.Has("RecordId") {
			isValid = func(r *EdgeOneRecord) bool { return r.RecordId == params.Get("RecordId") }
		} else {
			isValid = func(r *EdgeOneRecord) bool {
				return r.Status == "enable" || r.Status == "disable" && r.Content == ipAddr
			}
		}
		var recordSelected *EdgeOneRecord
		for i := range recordResult.Response.DnsRecords {
			r := &recordResult.Response.DnsRecords[i]
			if isValid(r) {
				recordSelected = r
				break
			}
		}
		if recordSelected != nil {
			// modifyrecord
			eo.modify(*recordSelected, domain, recordType, ipAddr, zoneId)
		} else {
			// addrecord
			eo.create(domain, recordType, ipAddr, zoneId)
		}
	}
}

// CreateDnsRecord https://cloud.tencent.com/document/product/1552/80720
func (eo *EdgeOne) create(domain *config.Domain, recordType string, ipAddr string, ZoneId string) {
	d := domain.DomainName
	if domain.SubDomain != "" && domain.SubDomain != "@" {
		d = domain.SubDomain + "." + domain.DomainName
	}
	asciiDomain, _ := idna.ToASCII(d)
	record := &EdgeOneRecord{
		ZoneId:   ZoneId,
		Name:     asciiDomain,
		Type:     recordType,
		Content:  ipAddr,
		Location: eo.getLocation(domain),
		TTL:      eo.TTL,
	}
	var status EdgeOneStatus
	err := eo.request(
		"CreateDnsRecord",
		record,
		&status,
	)

	if err != nil {
		util.Log("Failed to add domain %s! Result: %s", domain, err)
		domain.UpdateStatus = config.UpdatedFailed
		return
	}

	if status.Response.Error.Code == "" {
		util.Log("Added domain %s successfully! IP: %s", domain, ipAddr)
		domain.UpdateStatus = config.UpdatedSuccess
	} else {
		util.Log("Failed to add domain %s! Result: %s", domain, status.Response.Error.Message)
		domain.UpdateStatus = config.UpdatedFailed
	}
}

// ModifyDnsRecords https://cloud.tencent.com/document/product/1552/114252
func (eo *EdgeOne) modify(record EdgeOneRecord, domain *config.Domain, recordType string, ipAddr string, ZoneId string) {
	// skip if unchanged
	if record.Content == ipAddr {
		util.Log("Your's IP %s has not changed! Domain: %s", ipAddr, domain)
		return
	}
	var status EdgeOneStatus
	d := domain.DomainName
	if domain.SubDomain != "" && domain.SubDomain != "@" {
		d = domain.SubDomain + "." + domain.DomainName
	}
	asciiDomain, _ := idna.ToASCII(d)
	record.ZoneId = ZoneId
	record.Name = asciiDomain
	record.Type = recordType
	record.Content = ipAddr
	record.Location = eo.getLocation(domain)
	record.TTL = eo.TTL

	err := eo.request(
		"ModifyDnsRecords",
		struct {
			ZoneId     string          `json:"ZoneId"`
			DnsRecords []EdgeOneRecord `json:"DnsRecords"`
		}{
			ZoneId:     ZoneId,
			DnsRecords: []EdgeOneRecord{record},
		},
		&status,
	)

	if err != nil {
		util.Log("Failed to updated domain %s! Result: %s", domain, err)
		domain.UpdateStatus = config.UpdatedFailed
		return
	}

	if status.Response.Error.Code == "" {
		util.Log("Updated domain %s successfully! IP: %s", domain, ipAddr)
		domain.UpdateStatus = config.UpdatedSuccess
	} else {
		util.Log("Failed to updated domain %s! Result: %s", domain, status.Response.Error.Message)
		domain.UpdateStatus = config.UpdatedFailed
	}
}

func (eo *EdgeOne) getZone(domain string) (result EdgeOneZoneResponse, err error) {
	asciiDomain, _ := idna.ToASCII(domain)
	record := EdgeOneDescribeDns{
		Filters: []Filter{
			{Name: "zone-name", Values: []string{asciiDomain}},
		},
	}
	err = eo.request(
		"DescribeZones",
		record,
		&result,
	)
	return
}

// DescribeDnsRecords https://cloud.tencent.com/document/product/1552/80716
func (eo *EdgeOne) getRecordList(domain *config.Domain, recordType string, ZoneId string) (result EdgeOneRecordResponse, err error) {
	d := domain.DomainName
	if domain.SubDomain != "" && domain.SubDomain != "@" {
		d = domain.SubDomain + "." + domain.DomainName
	}
	asciiDomain, _ := idna.ToASCII(d)
	record := EdgeOneDescribeDns{
		ZoneId: ZoneId,
		Filters: []Filter{
			{Name: "name", Values: []string{asciiDomain}},
			{Name: "type", Values: []string{recordType}},
		},
	}

	err = eo.request(
		"DescribeDnsRecords",
		record,
		&result,
	)

	return
}

// getLocation get records default
func (eo *EdgeOne) getLocation(domain *config.Domain) string {
	if domain.GetCustomParams().Has("Location") {
		return domain.GetCustomParams().Get("Location")
	}
	return "Default"
}

// request shared request method
func (eo *EdgeOne) request(action string, data interface{}, result interface{}) (err error) {
	jsonStr := make([]byte, 0)
	if data != nil {
		jsonStr, _ = json.Marshal(data)
	}
	req, err := http.NewRequest(
		"POST",
		edgeoneEndPoint,
		bytes.NewBuffer(jsonStr),
	)
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-TC-Version", edgeoneVersion)

	util.TencentCloudSigner(eo.DNS.ID, eo.DNS.Secret, req, action, string(jsonStr), util.EdgeOne)

	client := util.CreateHTTPClient()
	resp, err := client.Do(req)
	err = util.GetHTTPResponse(resp, err, result)

	return
}
