package dns

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/jeessy2/ddns-go/v6/config"
	"github.com/jeessy2/ddns-go/v6/util"
)

const (
	huaweicloudEndpoint string = "https://dns.myhuaweicloud.com"
)

// https://support.huaweicloud.com/api-dns/dns_api_64001.html
// Huaweicloud Huaweicloud
type Huaweicloud struct {
	DNS     config.DNS
	Domains config.Domains
	TTL     int
}

// HuaweicloudZonesResp zones response
type HuaweicloudZonesResp struct {
	Zones []struct {
		ID         string
		Name       string
		Recordsets []HuaweicloudRecordsets
	}
}

// HuaweicloudRecordsResp record response
type HuaweicloudRecordsResp struct {
	Recordsets []HuaweicloudRecordsets
}

// HuaweicloudRecordsets record
type HuaweicloudRecordsets struct {
	ID      string
	Name    string `json:"name"`
	ZoneID  string `json:"zone_id"`
	Status  string
	Type    string   `json:"type"`
	TTL     int      `json:"ttl"`
	Records []string `json:"records"`
	Weight  int      `json:"weight"`
}

// Init
func (hw *Huaweicloud) Init(dnsConf *config.DnsConfig, ipv4cache *util.IpCache, ipv6cache *util.IpCache) {
	hw.Domains.Ipv4Cache = ipv4cache
	hw.Domains.Ipv6Cache = ipv6cache
	hw.DNS = dnsConf.DNS
	hw.Domains.GetNewIp(dnsConf)
	if dnsConf.TTL == "" {
		// default300s
		hw.TTL = 300
	} else {
		ttl, err := strconv.Atoi(dnsConf.TTL)
		if err != nil {
			hw.TTL = 300
		} else {
			hw.TTL = ttl
		}
	}
}

// AddUpdateDomainRecords add or update IPv4/IPv6 records
func (hw *Huaweicloud) AddUpdateDomainRecords() config.Domains {
	hw.addUpdateDomainRecords("A")
	hw.addUpdateDomainRecords("AAAA")
	return hw.Domains
}

func (hw *Huaweicloud) addUpdateDomainRecords(recordType string) {
	ipAddr, domains := hw.Domains.GetNewIpResult(recordType)

	if ipAddr == "" {
		return
	}

	for _, domain := range domains {
		customParams := domain.GetCustomParams()
		params := url.Values{}
		params.Set("name", domain.String())
		params.Set("type", recordType)

		//
		// record https://support.huaweicloud.com/api-dns/dns_api_64002.html
		if customParams.Has("zone_id") && customParams.Has("recordset_id") {
			var record HuaweicloudRecordsets
			err := hw.request(
				"GET",
				fmt.Sprintf(huaweicloudEndpoint+"/v2.1/zones/%s/recordsets/%s", customParams.Get("zone_id"), customParams.Get("recordset_id")),
				params,
				&record,
			)

			if err != nil {
				util.Log("Failed to query domain info! %s", err)
				domain.UpdateStatus = config.UpdatedFailed
				return
			}

			// update
			hw.modify(record, domain, ipAddr)

		} else { // parameters record https://support.huaweicloud.com/api-dns/dns_api_64003.html
			// parameters
			util.CopyUrlParams(customParams, params, nil)
			// parameters
			if params.Has("recordset_id") {
				params.Set("id", params.Get("recordset_id"))
				params.Del("recordset_id")
			}

			var records HuaweicloudRecordsResp
			err := hw.request(
				"GET",
				huaweicloudEndpoint+"/v2.1/recordsets",
				params,
				&records,
			)

			if err != nil {
				util.Log("Failed to query domain info! %s", err)
				domain.UpdateStatus = config.UpdatedFailed
				return
			}

			find := false
			for _, record := range records.Recordsets {
				// update default
				if record.Name == domain.String()+"." {
					// update
					hw.modify(record, domain, ipAddr)
					find = true
					break
				}
			}

			if !find {
				thIdParamName := ""
				if customParams.Has("id") {
					thIdParamName = "id"
				} else if customParams.Has("recordset_id") {
					thIdParamName = "recordset_id"
				}

				if thIdParamName != "" {
					util.Log("DNS resolution for domain %s was not found, and the creation failed due to the added parameter %s=%s. This update has been ignored.", domain, thIdParamName, customParams.Get(thIdParamName))
				} else {
					// add
					hw.create(domain, recordType, ipAddr)
				}
			}
		}
	}
}

// create
func (hw *Huaweicloud) create(domain *config.Domain, recordType string, ipAddr string) {
	zone, err := hw.getZones(domain)
	if err != nil {
		util.Log("Failed to query domain info! %s", err)
		domain.UpdateStatus = config.UpdatedFailed
		return
	}

	if len(zone.Zones) == 0 {
		util.Log("Root domain not found in DNS provider: %s", domain.DomainName)
		domain.UpdateStatus = config.UpdatedFailed
		return
	}

	zoneID := zone.Zones[0].ID
	for _, z := range zone.Zones {
		if z.Name == domain.DomainName+"." {
			zoneID = z.ID
			break
		}
	}

	record := &HuaweicloudRecordsets{
		Type:    recordType,
		Name:    domain.String() + ".",
		Records: []string{ipAddr},
		TTL:     hw.TTL,
		Weight:  1,
	}
	var result HuaweicloudRecordsets
	err = hw.request(
		"POST",
		fmt.Sprintf(huaweicloudEndpoint+"/v2.1/zones/%s/recordsets", zoneID),
		record,
		&result,
	)

	if err != nil {
		util.Log("Failed to add domain %s! Result: %s", domain, err)
		domain.UpdateStatus = config.UpdatedFailed
		return
	}

	if len(result.Records) > 0 && result.Records[0] == ipAddr {
		util.Log("Added domain %s successfully! IP: %s", domain, ipAddr)
		domain.UpdateStatus = config.UpdatedSuccess
	} else {
		util.Log("Failed to add domain %s! Result: %s", domain, result.Status)
		domain.UpdateStatus = config.UpdatedFailed
	}
}

// modify
func (hw *Huaweicloud) modify(record HuaweicloudRecordsets, domain *config.Domain, ipAddr string) {

	// skip if unchanged
	if len(record.Records) > 0 && record.Records[0] == ipAddr {
		util.Log("Your's IP %s has not changed! Domain: %s", ipAddr, domain)
		return
	}

	var request = make(map[string]interface{})
	request["name"] = record.Name
	request["type"] = record.Type
	request["records"] = []string{ipAddr}
	request["ttl"] = hw.TTL

	var result HuaweicloudRecordsets

	err := hw.request(
		"PUT",
		fmt.Sprintf(huaweicloudEndpoint+"/v2.1/zones/%s/recordsets/%s", record.ZoneID, record.ID),
		&request,
		&result,
	)

	if err != nil {
		util.Log("Failed to updated domain %s! Result: %s", domain, err)
		domain.UpdateStatus = config.UpdatedFailed
		return
	}

	if len(result.Records) > 0 && result.Records[0] == ipAddr {
		util.Log("Updated domain %s successfully! IP: %s", domain, ipAddr)
		domain.UpdateStatus = config.UpdatedSuccess
	} else {
		util.Log("Failed to updated domain %s! Result: %s", domain, result.Status)
		domain.UpdateStatus = config.UpdatedFailed
	}
}

// get domain record list
func (hw *Huaweicloud) getZones(domain *config.Domain) (result HuaweicloudZonesResp, err error) {
	err = hw.request(
		"GET",
		huaweicloudEndpoint+"/v2/zones",
		url.Values{"name": []string{domain.DomainName}},
		&result,
	)

	return
}

// request shared request method
func (hw *Huaweicloud) request(method string, urlString string, data interface{}, result interface{}) (err error) {
	var (
		req *http.Request
	)

	if method == "GET" {
		req, err = http.NewRequest(
			method,
			urlString,
			bytes.NewBuffer(nil),
		)

		req.URL.RawQuery = data.(url.Values).Encode()
	} else {
		jsonStr := make([]byte, 0)
		if data != nil {
			jsonStr, _ = json.Marshal(data)
		}

		req, err = http.NewRequest(
			method,
			urlString,
			bytes.NewBuffer(jsonStr),
		)
	}

	if err != nil {
		return
	}

	s := util.Signer{
		Key:    hw.DNS.ID,
		Secret: hw.DNS.Secret,
	}
	s.Sign(req)

	req.Header.Add("content-type", "application/json")

	client := util.CreateHTTPClient()
	resp, err := client.Do(req)
	err = util.GetHTTPResponse(resp, err, result)

	return
}
