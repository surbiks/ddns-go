package dns

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/jeessy2/ddns-go/v6/config"
	"github.com/jeessy2/ddns-go/v6/util"
)

// https://cloud.baidu.com/doc/BCD/s/4jwvymhs7

const (
	baiduEndpoint = "https://bcd.baidubce.com"
)

type BaiduCloud struct {
	DNS     config.DNS
	Domains config.Domains
	TTL     int
}

// BaiduRecord parserecord
type BaiduRecord struct {
	RecordId uint   `json:"recordId"`
	Domain   string `json:"domain"`
	View     string `json:"view"`
	Rdtype   string `json:"rdtype"`
	TTL      int    `json:"ttl"`
	Rdata    string `json:"rdata"`
	ZoneName string `json:"zoneName"`
	Status   string `json:"status"`
}

// BaiduRecordsResp getparse result
type BaiduRecordsResp struct {
	TotalCount int           `json:"totalCount"`
	Result     []BaiduRecord `json:"result"`
}

// BaiduListRequest getparse request body json
type BaiduListRequest struct {
	Domain   string `json:"domain"`
	PageNum  int    `json:"pageNum"`
	PageSize int    `json:"pageSize"`
}

// BaiduModifyRequest modifyparserequest body json
type BaiduModifyRequest struct {
	RecordId uint   `json:"recordId"`
	Domain   string `json:"domain"`
	View     string `json:"view"`
	RdType   string `json:"rdType"`
	TTL      int    `json:"ttl"`
	Rdata    string `json:"rdata"`
	ZoneName string `json:"zoneName"`
}

// BaiduCreateRequest create parserequest body json
type BaiduCreateRequest struct {
	Domain   string `json:"domain"`
	RdType   string `json:"rdType"`
	TTL      int    `json:"ttl"`
	Rdata    string `json:"rdata"`
	ZoneName string `json:"zoneName"`
}

func (baidu *BaiduCloud) Init(dnsConf *config.DnsConfig, ipv4cache *util.IpCache, ipv6cache *util.IpCache) {
	baidu.Domains.Ipv4Cache = ipv4cache
	baidu.Domains.Ipv6Cache = ipv6cache
	baidu.DNS = dnsConf.DNS
	baidu.Domains.GetNewIp(dnsConf)
	if dnsConf.TTL == "" {
		// default300s
		baidu.TTL = 300
	} else {
		ttl, err := strconv.Atoi(dnsConf.TTL)
		if err != nil {
			baidu.TTL = 300
		} else {
			baidu.TTL = ttl
		}
	}
}

// AddUpdateDomainRecords add or update IPv4/IPv6 records
func (baidu *BaiduCloud) AddUpdateDomainRecords() config.Domains {
	baidu.addUpdateDomainRecords("A")
	baidu.addUpdateDomainRecords("AAAA")
	return baidu.Domains
}

func (baidu *BaiduCloud) addUpdateDomainRecords(recordType string) {
	ipAddr, domains := baidu.Domains.GetNewIpResult(recordType)
	if ipAddr == "" {
		return
	}

	for _, domain := range domains {
		var records BaiduRecordsResp

		requestBody := BaiduListRequest{
			Domain:   domain.DomainName,
			PageNum:  1,
			PageSize: 1000,
		}

		err := baidu.request("POST", baiduEndpoint+"/v1/domain/resolve/list", requestBody, &records)
		if err != nil {
			util.Log("Failed to query domain info! %s", err)
			domain.UpdateStatus = config.UpdatedFailed
			return
		}

		find := false
		for _, record := range records.Result {
			if record.Domain == domain.GetSubDomain() {
				// update
				baidu.modify(record, domain, recordType, ipAddr)
				find = true
				break
			}
		}
		if !find {
			// create
			baidu.create(domain, recordType, ipAddr)
		}
	}
}

// create create parse
func (baidu *BaiduCloud) create(domain *config.Domain, recordType string, ipAddr string) {
	var baiduCreateRequest = BaiduCreateRequest{
		Domain:   domain.GetSubDomain(), //handle @
		RdType:   recordType,
		TTL:      baidu.TTL,
		Rdata:    ipAddr,
		ZoneName: domain.DomainName,
	}
	var result BaiduRecordsResp

	err := baidu.request("POST", baiduEndpoint+"/v1/domain/resolve/add", baiduCreateRequest, &result)
	if err == nil {
		util.Log("Added domain %s successfully! IP: %s", domain, ipAddr)
		domain.UpdateStatus = config.UpdatedSuccess
	} else {
		util.Log("Failed to add domain %s! Result: %s", domain, err)
		domain.UpdateStatus = config.UpdatedFailed
	}
}

// modify updateparse
func (baidu *BaiduCloud) modify(record BaiduRecord, domain *config.Domain, rdType string, ipAddr string) {
	//
	if record.Rdata == ipAddr {
		util.Log("Your's IP %s has not changed! Domain: %s", ipAddr, domain)
		return
	}
	var baiduModifyRequest = BaiduModifyRequest{
		RecordId: record.RecordId,
		Domain:   record.Domain,
		View:     record.View,
		RdType:   rdType,
		TTL:      record.TTL,
		Rdata:    ipAddr,
		ZoneName: record.ZoneName,
	}
	var result BaiduRecordsResp

	err := baidu.request("POST", baiduEndpoint+"/v1/domain/resolve/edit", baiduModifyRequest, &result)
	if err == nil {
		util.Log("Updated domain %s successfully! IP: %s", domain, ipAddr)
		domain.UpdateStatus = config.UpdatedSuccess
	} else {
		util.Log("Failed to updated domain %s! Result: %s", domain, err)
		domain.UpdateStatus = config.UpdatedFailed
	}
}

// request shared request method
func (baidu *BaiduCloud) request(method string, url string, data interface{}, result interface{}) (err error) {
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

	util.BaiduSigner(baidu.DNS.ID, baidu.DNS.Secret, req)

	client := util.CreateHTTPClient()
	resp, err := client.Do(req)
	err = util.GetHTTPResponse(resp, err, result)

	return
}
