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

const arvancloudAPIEndpoint = "https://napi.arvancloud.ir/cdn/4.0"

// Arvancloud ArvanCloud DNS implementation.
type Arvancloud struct {
	DNS     config.DNS
	Domains config.Domains
	TTL     int
}

type arvancloudRecordsResp struct {
	Data []arvancloudRecord `json:"data"`
}

type arvancloudRecord struct {
	ID    interface{}   `json:"id"`
	Name  string        `json:"name"`
	Type  string        `json:"type"`
	Value []interface{} `json:"value"`
}

type arvancloudRecordReq struct {
	Type  string              `json:"type"`
	Name  string              `json:"name"`
	Value []map[string]string `json:"value"`
	TTL   int                 `json:"ttl"`
}

// Init initializes provider config.
func (ac *Arvancloud) Init(dnsConf *config.DnsConfig, ipv4cache *util.IpCache, ipv6cache *util.IpCache) {
	ac.Domains.Ipv4Cache = ipv4cache
	ac.Domains.Ipv6Cache = ipv6cache
	ac.DNS = dnsConf.DNS
	ac.Domains.GetNewIp(dnsConf)
	if dnsConf.TTL == "" {
		ac.TTL = 120
	} else {
		ttl, err := strconv.Atoi(dnsConf.TTL)
		if err != nil {
			ac.TTL = 120
		} else {
			ac.TTL = ttl
		}
	}
}

// AddUpdateDomainRecords adds or updates IPv4/IPv6 records.
func (ac *Arvancloud) AddUpdateDomainRecords() config.Domains {
	ac.addUpdateDomainRecords("A")
	ac.addUpdateDomainRecords("AAAA")
	return ac.Domains
}

func (ac *Arvancloud) addUpdateDomainRecords(recordType string) {
	ipAddr, domains := ac.Domains.GetNewIpResult(recordType)
	if ipAddr == "" {
		return
	}

	for _, domain := range domains {
		recordsResp, err := ac.getRecordList(domain, recordType)
		if err != nil {
			util.Log("Failed to query domain info! %s", err)
			domain.UpdateStatus = config.UpdatedFailed
			continue
		}

		recordName := domain.GetSubDomain()
		record := findArvancloudRecord(recordsResp.Data, recordName, recordType)
		if record == nil {
			ac.create(domain, recordType, ipAddr)
			continue
		}

		ac.modify(*record, domain, recordType, ipAddr)
	}
}

func (ac *Arvancloud) create(domain *config.Domain, recordType, ipAddr string) {
	req := arvancloudRecordReq{
		Type:  recordType,
		Name:  domain.GetSubDomain(),
		Value: []map[string]string{{"ip": ipAddr}},
		TTL:   ac.TTL,
	}

	var result interface{}
	err := ac.request(
		"POST",
		fmt.Sprintf("%s/domains/%s/dns-records", arvancloudAPIEndpoint, domain.DomainName),
		req,
		&result,
	)

	if err != nil {
		util.Log("Failed to add domain %s! Result: %s", domain, err)
		domain.UpdateStatus = config.UpdatedFailed
		return
	}

	util.Log("Added domain %s successfully! IP: %s", domain, ipAddr)
	domain.UpdateStatus = config.UpdatedSuccess
}

func (ac *Arvancloud) modify(record arvancloudRecord, domain *config.Domain, recordType, ipAddr string) {
	recordID := getArvancloudRecordID(record.ID)
	if recordID == "" {
		util.Log("Failed to update DNS record %s! Exception: empty record id", domain)
		domain.UpdateStatus = config.UpdatedFailed
		return
	}

	if getArvancloudRecordIP(record) == ipAddr {
		util.Log("Your's IP %s has not changed! Domain: %s", ipAddr, domain)
		return
	}

	req := arvancloudRecordReq{
		Type:  recordType,
		Name:  domain.GetSubDomain(),
		Value: []map[string]string{{"ip": ipAddr}},
		TTL:   ac.TTL,
	}

	var result interface{}
	err := ac.request(
		"PUT",
		fmt.Sprintf("%s/domains/%s/dns-records/%s", arvancloudAPIEndpoint, domain.DomainName, recordID),
		req,
		&result,
	)

	if err != nil {
		util.Log("Failed to updated domain %s! Result: %s", domain, err)
		domain.UpdateStatus = config.UpdatedFailed
		return
	}

	util.Log("Updated domain %s successfully! IP: %s", domain, ipAddr)
	domain.UpdateStatus = config.UpdatedSuccess
}

func (ac *Arvancloud) getRecordList(domain *config.Domain, recordType string) (result arvancloudRecordsResp, err error) {
	params := url.Values{}
	params.Set("search", domain.GetSubDomain())
	params.Set("type", recordType)

	err = ac.request(
		"GET",
		fmt.Sprintf("%s/domains/%s/dns-records?%s", arvancloudAPIEndpoint, domain.DomainName, params.Encode()),
		nil,
		&result,
	)
	return
}

func (ac *Arvancloud) request(method, reqURL string, data interface{}, result interface{}) (err error) {
	jsonStr := make([]byte, 0)
	if data != nil {
		jsonStr, _ = json.Marshal(data)
	}

	req, err := http.NewRequest(method, reqURL, bytes.NewBuffer(jsonStr))
	if err != nil {
		return
	}

	req.Header.Set("Authorization", "apikey "+ac.DNS.Secret)
	req.Header.Set("Content-Type", "application/json")

	client := util.CreateHTTPClient()
	resp, err := client.Do(req)
	err = util.GetHTTPResponse(resp, err, result)
	return
}

func findArvancloudRecord(records []arvancloudRecord, name, recordType string) *arvancloudRecord {
	for i := range records {
		if strings.EqualFold(records[i].Name, name) && strings.EqualFold(records[i].Type, recordType) {
			return &records[i]
		}
	}
	return nil
}

func getArvancloudRecordID(id interface{}) string {
	if id == nil {
		return ""
	}
	switch v := id.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatInt(int64(v), 10)
	default:
		return fmt.Sprint(v)
	}
}

func getArvancloudRecordIP(record arvancloudRecord) string {
	for _, v := range record.Value {
		if m, ok := v.(map[string]interface{}); ok {
			if ip, ok := m["ip"].(string); ok {
				return ip
			}
		}
		if ip, ok := v.(string); ok {
			return ip
		}
	}
	return ""
}
