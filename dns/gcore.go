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

const gcoreAPIEndpoint = "https://api.gcore.com/dns/v2"

// Gcore Gcore DNS
type Gcore struct {
	DNS     config.DNS
	Domains config.Domains
	TTL     int
}

// GcoreZoneResponse zones result
type GcoreZoneResponse struct {
	Zones       []GcoreZone `json:"zones"`
	TotalAmount int         `json:"total_amount"`
}

// GcoreZone domain
type GcoreZone struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// GcoreRRSetListResponse RRSet result
type GcoreRRSetListResponse struct {
	RRSets      []GcoreRRSet `json:"rrsets"`
	TotalAmount int          `json:"total_amount"`
}

// GcoreRRSet RRSetrecord entity
type GcoreRRSet struct {
	Name            string                 `json:"name"`
	Type            string                 `json:"type"`
	TTL             int                    `json:"ttl"`
	ResourceRecords []GcoreResourceRecord  `json:"resource_records"`
	Meta            map[string]interface{} `json:"meta,omitempty"`
}

// GcoreResourceRecord record
type GcoreResourceRecord struct {
	Content []interface{}          `json:"content"`
	Enabled bool                   `json:"enabled"`
	ID      int                    `json:"id,omitempty"`
	Meta    map[string]interface{} `json:"meta,omitempty"`
}

// GcoreInputRRSet RRSet
type GcoreInputRRSet struct {
	TTL             int                        `json:"ttl"`
	ResourceRecords []GcoreInputResourceRecord `json:"resource_records"`
	Meta            map[string]interface{}     `json:"meta,omitempty"`
}

// GcoreInputResourceRecord record
type GcoreInputResourceRecord struct {
	Content []interface{}          `json:"content"`
	Enabled bool                   `json:"enabled"`
	Meta    map[string]interface{} `json:"meta,omitempty"`
}

// Init
func (gc *Gcore) Init(dnsConf *config.DnsConfig, ipv4cache *util.IpCache, ipv6cache *util.IpCache) {
	gc.Domains.Ipv4Cache = ipv4cache
	gc.Domains.Ipv6Cache = ipv6cache
	gc.DNS = dnsConf.DNS
	gc.Domains.GetNewIp(dnsConf)
	if dnsConf.TTL == "" {
		// default 120
		gc.TTL = 120
	} else {
		ttl, err := strconv.Atoi(dnsConf.TTL)
		if err != nil {
			gc.TTL = 120
		} else {
			gc.TTL = ttl
		}
	}
}

// AddUpdateDomainRecords add or update IPv4/IPv6 records
func (gc *Gcore) AddUpdateDomainRecords() config.Domains {
	gc.addUpdateDomainRecords("A")
	gc.addUpdateDomainRecords("AAAA")
	return gc.Domains
}

func (gc *Gcore) addUpdateDomainRecords(recordType string) {
	ipAddr, domains := gc.Domains.GetNewIpResult(recordType)

	if ipAddr == "" {
		return
	}

	for _, domain := range domains {
		// get zone
		zoneInfo, err := gc.getZoneByDomain(domain)
		if err != nil {
			util.Log("Failed to query domain info! %s", err)
			domain.UpdateStatus = config.UpdatedFailed
			continue
		}

		if zoneInfo == nil {
			util.Log("Root domain not found in DNS provider: %s", domain.DomainName)
			domain.UpdateStatus = config.UpdatedFailed
			continue
		}

		// record
		existingRecord, err := gc.getRRSet(zoneInfo.Name, domain.GetSubDomain(), recordType)
		if err != nil {
			util.Log("Failed to query domain info! %s", err)
			domain.UpdateStatus = config.UpdatedFailed
			continue
		}

		if existingRecord != nil {
			// update record
			gc.updateRecord(zoneInfo.Name, domain, recordType, ipAddr, existingRecord)
		} else {
			// create record
			gc.createRecord(zoneInfo.Name, domain, recordType, ipAddr)
		}
	}
}

// getdomain Zone
func (gc *Gcore) getZoneByDomain(domain *config.Domain) (*GcoreZone, error) {
	var result GcoreZoneResponse
	params := url.Values{}
	params.Set("name", domain.DomainName)

	err := gc.request(
		"GET",
		fmt.Sprintf("%s/zones?%s", gcoreAPIEndpoint, params.Encode()),
		nil,
		&result,
	)

	if err != nil {
		return nil, err
	}

	if len(result.Zones) > 0 {
		return &result.Zones[0], nil
	}

	return nil, nil
}

// get RRSetrecord
func (gc *Gcore) getRRSet(zoneName, recordName, recordType string) (*GcoreRRSet, error) {
	var result GcoreRRSetListResponse

	err := gc.request(
		"GET",
		fmt.Sprintf("%s/zones/%s/rrsets", gcoreAPIEndpoint, zoneName),
		nil,
		&result,
	)

	if err != nil {
		return nil, err
	}

	// record
	fullRecordName := recordName
	if recordName != "" && recordName != "@" {
		fullRecordName = recordName + "." + zoneName
	} else {
		fullRecordName = zoneName
	}

	for _, rrset := range result.RRSets {
		if rrset.Name == fullRecordName && rrset.Type == recordType {
			return &rrset, nil
		}
	}

	return nil, nil
}

// create record
func (gc *Gcore) createRecord(zoneName string, domain *config.Domain, recordType string, ipAddr string) {
	recordName := domain.GetSubDomain()
	if recordName == "" || recordName == "@" {
		recordName = zoneName
	} else {
		recordName = recordName + "." + zoneName
	}

	inputRRSet := GcoreInputRRSet{
		TTL: gc.TTL,
		ResourceRecords: []GcoreInputResourceRecord{
			{
				Content: []interface{}{ipAddr},
				Enabled: true,
			},
		},
	}

	var result interface{}
	err := gc.request(
		"POST",
		fmt.Sprintf("%s/zones/%s/%s/%s", gcoreAPIEndpoint, zoneName, recordName, recordType),
		inputRRSet,
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

// update record
func (gc *Gcore) updateRecord(zoneName string, domain *config.Domain, recordType string, ipAddr string, existingRecord *GcoreRRSet) {
	// checkIP
	if len(existingRecord.ResourceRecords) > 0 && len(existingRecord.ResourceRecords[0].Content) > 0 {
		if existingRecord.ResourceRecords[0].Content[0] == ipAddr {
			util.Log("Your's IP %s has not changed! Domain: %s", ipAddr, domain)
			return
		}
	}

	recordName := domain.GetSubDomain()
	if recordName == "" || recordName == "@" {
		recordName = zoneName
	} else {
		recordName = recordName + "." + zoneName
	}

	inputRRSet := GcoreInputRRSet{
		TTL: gc.TTL,
		ResourceRecords: []GcoreInputResourceRecord{
			{
				Content: []interface{}{ipAddr},
				Enabled: true,
			},
		},
	}

	var result interface{}
	err := gc.request(
		"PUT",
		fmt.Sprintf("%s/zones/%s/%s/%s", gcoreAPIEndpoint, zoneName, recordName, recordType),
		inputRRSet,
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

// request shared request method
func (gc *Gcore) request(method string, url string, data interface{}, result interface{}) (err error) {
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

	req.Header.Set("Authorization", "APIKey "+gc.DNS.Secret)
	req.Header.Set("Content-Type", "application/json")

	client := util.CreateHTTPClient()
	resp, err := client.Do(req)
	err = util.GetHTTPResponse(resp, err, result)

	return
}
