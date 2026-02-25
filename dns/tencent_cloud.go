package dns

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/jeessy2/ddns-go/v6/config"
	"github.com/jeessy2/ddns-go/v6/util"
)

const (
	tencentCloudEndPoint = "https://dnspod.tencentcloudapi.com"
	tencentCloudVersion  = "2021-03-23"
)

// TencentCloud DNSPod API 3.0
// https://cloud.tencent.com/document/api/1427/56193
type TencentCloud struct {
	DNS     config.DNS
	Domains config.Domains
	TTL     int
}

// TencentCloudRecord record
type TencentCloudRecord struct {
	Domain string `json:"Domain"`
	// DescribeRecordList SubDomain
	SubDomain string `json:"SubDomain,omitempty"`
	// CreateRecord/ModifyRecord Subdomain
	Subdomain  string `json:"Subdomain,omitempty"`
	RecordType string `json:"RecordType"`
	RecordLine string `json:"RecordLine"`
	// DescribeRecordList Value
	Value string `json:"Value,omitempty"`
	// CreateRecord/DescribeRecordList RecordId
	RecordId int64 `json:"RecordId,omitempty"`
	// DescribeRecordList TTL
	TTL int `json:"TTL,omitempty"`
}

// TencentCloudRecordListsResp response for domain record list
type TencentCloudRecordListsResp struct {
	TencentCloudStatus
	Response struct {
		RecordCountInfo struct {
			TotalCount int `json:"TotalCount"`
		} `json:"RecordCountInfo"`

		RecordList []TencentCloudRecord `json:"RecordList"`
	}
}

// TencentCloudStatus status
// https://cloud.tencent.com/document/product/1427/56192
type TencentCloudStatus struct {
	Response struct {
		Error struct {
			Code    string
			Message string
		}
	}
}

func (tc *TencentCloud) Init(dnsConf *config.DnsConfig, ipv4cache *util.IpCache, ipv6cache *util.IpCache) {
	tc.Domains.Ipv4Cache = ipv4cache
	tc.Domains.Ipv6Cache = ipv6cache
	tc.DNS = dnsConf.DNS
	tc.Domains.GetNewIp(dnsConf)
	if dnsConf.TTL == "" {
		// default 600s
		tc.TTL = 600
	} else {
		ttl, err := strconv.Atoi(dnsConf.TTL)
		if err != nil {
			tc.TTL = 600
		} else {
			tc.TTL = ttl
		}
	}
}

// AddUpdateDomainRecords add or update IPv4/IPv6 records
func (tc *TencentCloud) AddUpdateDomainRecords() config.Domains {
	tc.addUpdateDomainRecords("A")
	tc.addUpdateDomainRecords("AAAA")
	return tc.Domains
}

func (tc *TencentCloud) addUpdateDomainRecords(recordType string) {
	ipAddr, domains := tc.Domains.GetNewIpResult(recordType)

	if ipAddr == "" {
		return
	}

	for _, domain := range domains {
		result, err := tc.getRecordList(domain, recordType)
		if err != nil {
			util.Log("Failed to query domain info! %s", err)
			domain.UpdateStatus = config.UpdatedFailed
			return
		}

		if result.Response.RecordCountInfo.TotalCount > 0 {
			// first by default
			recordSelected := result.Response.RecordList[0]
			params := domain.GetCustomParams()
			if params.Has("RecordId") {
				for i := 0; i < result.Response.RecordCountInfo.TotalCount; i++ {
					if strconv.FormatInt(result.Response.RecordList[i].RecordId, 10) == params.Get("RecordId") {
						recordSelected = result.Response.RecordList[i]
					}
				}
			}

			// modifyrecord
			tc.modify(recordSelected, domain, recordType, ipAddr)
		} else {
			// addrecord
			tc.create(domain, recordType, ipAddr)
		}
	}
}

// create addrecord
// CreateRecord https://cloud.tencent.com/document/api/1427/56180
func (tc *TencentCloud) create(domain *config.Domain, recordType string, ipAddr string) {
	record := &TencentCloudRecord{
		Domain:     domain.DomainName,
		SubDomain:  domain.GetSubDomain(),
		RecordType: recordType,
		RecordLine: tc.getRecordLine(domain),
		Value:      ipAddr,
		TTL:        tc.TTL,
	}

	var status TencentCloudStatus
	err := tc.request(
		"CreateRecord",
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

// modify modifyrecord
// ModifyRecord https://cloud.tencent.com/document/api/1427/56157
func (tc *TencentCloud) modify(record TencentCloudRecord, domain *config.Domain, recordType string, ipAddr string) {
	// skip if unchanged
	if record.Value == ipAddr {
		util.Log("Your's IP %s has not changed! Domain: %s", ipAddr, domain)
		return
	}
	var status TencentCloudStatus
	record.Domain = domain.DomainName
	record.SubDomain = domain.GetSubDomain()
	record.RecordType = recordType
	record.RecordLine = tc.getRecordLine(domain)
	record.Value = ipAddr
	record.TTL = tc.TTL
	err := tc.request(
		"ModifyRecord",
		record,
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

// getRecordList get domain record list
// DescribeRecordList https://cloud.tencent.com/document/api/1427/56166
func (tc *TencentCloud) getRecordList(domain *config.Domain, recordType string) (result TencentCloudRecordListsResp, err error) {
	record := TencentCloudRecord{
		Domain:     domain.DomainName,
		Subdomain:  domain.GetSubDomain(),
		RecordType: recordType,
		RecordLine: tc.getRecordLine(domain),
	}
	err = tc.request(
		"DescribeRecordList",
		record,
		&result,
	)

	return
}

// getRecordLine get records default
func (tc *TencentCloud) getRecordLine(domain *config.Domain) string {
	if domain.GetCustomParams().Has("RecordLine") {
		return domain.GetCustomParams().Get("RecordLine")
	}
	return "\u9ed8\u8ba4"
}

// request shared request method
func (tc *TencentCloud) request(action string, data interface{}, result interface{}) (err error) {
	jsonStr := make([]byte, 0)
	if data != nil {
		jsonStr, _ = json.Marshal(data)
	}
	req, err := http.NewRequest(
		"POST",
		tencentCloudEndPoint,
		bytes.NewBuffer(jsonStr),
	)
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-TC-Version", tencentCloudVersion)

	util.TencentCloudSigner(tc.DNS.ID, tc.DNS.Secret, req, action, string(jsonStr), util.DnsPod)

	client := util.CreateHTTPClient()
	resp, err := client.Do(req)
	err = util.GetHTTPResponse(resp, err, result)

	return
}
