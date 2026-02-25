package dns

import (
	"bytes"
	"encoding/json"
	"github.com/jeessy2/ddns-go/v6/config"
	"github.com/jeessy2/ddns-go/v6/util"
	"net/http"
	"strconv"
	"strings"
)

const (
	dynv6Endpoint = "https://dynv6.com"
)

type Dynv6 struct {
	DNS     config.DNS
	Domains config.Domains
	TTL     string
}

type Dynv6Zone struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
	Ipv4 string `json:"ipv4address"`
	Ipv6 string `json:"ipv6prefix"`
}

type Dynv6Record struct {
	ID     uint   `json:"id"`
	ZoneID uint   `json:"zoneID"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Data   string `json:"data"`
}

// Init
func (dynv6 *Dynv6) Init(dnsConf *config.DnsConfig, ipv4cache *util.IpCache, ipv6cache *util.IpCache) {
	dynv6.Domains.Ipv4Cache = ipv4cache
	dynv6.Domains.Ipv6Cache = ipv6cache
	dynv6.DNS = dnsConf.DNS
	dynv6.Domains.GetNewIp(dnsConf)
	if dnsConf.TTL == "" {
		// default600s
		dynv6.TTL = "600"
	} else {
		dynv6.TTL = dnsConf.TTL
	}
}

// AddUpdateDomainRecords add or update IPv4/IPv6 records
func (dynv6 *Dynv6) AddUpdateDomainRecords() config.Domains {
	dynv6.addUpdateDomainRecords("A")
	dynv6.addUpdateDomainRecords("AAAA")
	return dynv6.Domains
}

func (dynv6 *Dynv6) addUpdateDomainRecords(recordType string) {
	ipAddr, domains := dynv6.Domains.GetNewIpResult(recordType)

	if ipAddr == "" {
		return
	}

	for _, domain := range domains {
		isFindZone, findZone, isMain, err := dynv6.findZone(domain)

		if err != nil {
			util.Log("Failed to query domain info! %s", err)
			domain.UpdateStatus = config.UpdatedFailed
			return
		}

		if !isFindZone {
			util.Log("Root domain not found in DNS provider: %s", domain)
			domain.UpdateStatus = config.UpdatedFailed
			continue
		}

		zoneId := strconv.FormatUint(uint64(findZone.ID), 10)

		if isMain {
			// domain domain DNSrecord update
			if (recordType == "A" && findZone.Ipv4 == ipAddr) || (recordType == "AAAA" && findZone.Ipv6 == ipAddr) {
				// ip dnsservice update
				util.Log("Your's IP %s has not changed! Domain: %s", ipAddr, domain)
				domain.UpdateStatus = config.UpdatedNothing
			} else {
				dynv6.modifyMain(domain, zoneId, recordType, ipAddr)
			}
		} else {
			// domain check domainrecord updaterecord create

			// handlesubDomain
			processSubDomainOk := dynv6.processSubDomain(domain, findZone)

			if !processSubDomainOk {
				util.Log("The domain %s is incorrect", domain)
				domain.UpdateStatus = config.UpdatedFailed
				continue
			}

			isFindRecord, findRecord, err := dynv6.findRecord(domain, zoneId, recordType)

			if err != nil {
				util.Log("Failed to query domain info! %s", err)
				domain.UpdateStatus = config.UpdatedFailed
				return
			}

			if isFindRecord {
				// update
				if findRecord.Type == recordType && findRecord.Data == ipAddr {
					// ip dnsservice update
					util.Log("Your's IP %s has not changed! Domain: %s", ipAddr, domain)
					domain.UpdateStatus = config.UpdatedNothing
				} else {
					dynv6.modify(domain, zoneId, findRecord, recordType, ipAddr)
				}
			} else {
				// createrecord
				dynv6.create(domain, zoneId, recordType, ipAddr)
			}
		}
	}
}

func (dynv6 *Dynv6) processSubDomain(domain *config.Domain, zone Dynv6Zone) bool {
	// subDomain
	subDomainLen := len(domain.String()) - len(zone.Name) - 1
	if subDomainLen <= 0 {
		return false
	}
	subDomain := domain.String()[:subDomainLen]

	domain.DomainName = zone.Name
	domain.SubDomain = subDomain
	return true
}

// domaingetzone
func (dynv6 *Dynv6) findZone(domain *config.Domain) (isFind bool, zone Dynv6Zone, isMain bool, err error) {
	var zones []Dynv6Zone
	isFind = false
	isMain = false

	// get all zones
	err = dynv6.request("GET", dynv6Endpoint+"/api/v2/zones", nil, &zones)

	if err != nil {
		return
	}

	// token zone domain zone domain domain domain
	for _, z := range zones {
		if strings.HasSuffix(domain.String(), z.Name) {
			isFind = true
			zone = z
			if domain.String() == z.Name {
				isMain = true
			}
			break
		}
	}

	return
}

// domaingetrecord
func (dynv6 *Dynv6) findRecord(domain *config.Domain, zoneId string, recordType string) (isFind bool, record Dynv6Record, err error) {
	var records []Dynv6Record
	isFind = false

	err = dynv6.request("GET", dynv6Endpoint+"/api/v2/zones/"+zoneId+"/records", nil, &records)
	if err != nil {
		return
	}

	// zone record update create
	for _, r := range records {
		if r.Name == domain.SubDomain && r.Type == recordType {
			isFind = true
			record = r
			break
		}
	}

	return
}

// modify update domain
func (dynv6 *Dynv6) modifyMain(domain *config.Domain, zoneId string, recordType string, ipAddr string) {
	var zoneUpdateReq = Dynv6Zone{}
	if recordType == "A" {
		zoneUpdateReq.Ipv4 = ipAddr
	} else {
		zoneUpdateReq.Ipv6 = ipAddr
	}

	err := dynv6.request("PATCH", dynv6Endpoint+"/api/v2/zones/"+zoneId, zoneUpdateReq, &Dynv6Zone{})

	if err != nil {
		util.Log("Failed to updated domain %s! Result: %s", domain, err)
		domain.UpdateStatus = config.UpdatedFailed
	} else {
		util.Log("Updated domain %s successfully! IP: %s", domain, ipAddr)
		domain.UpdateStatus = config.UpdatedSuccess
	}
}

// create create parse
func (dynv6 *Dynv6) create(domain *config.Domain, zoneId string, recordType string, ipAddr string) {
	recordUpdateReq := Dynv6Record{
		Name: domain.SubDomain,
		Type: recordType,
		Data: ipAddr,
	}

	err := dynv6.request("POST", dynv6Endpoint+"/api/v2/zones/"+zoneId+"/records", recordUpdateReq, &Dynv6Record{})

	if err != nil {
		util.Log("Failed to add domain %s! Result: %s", domain, err)
		domain.UpdateStatus = config.UpdatedFailed
	} else {
		util.Log("Added domain %s successfully! IP: %s", domain, ipAddr)
		domain.UpdateStatus = config.UpdatedSuccess
	}
}

// modify updateparse
func (dynv6 *Dynv6) modify(domain *config.Domain, zoneId string, record Dynv6Record, recordType string, ipAddr string) {
	record.Type = recordType
	record.Data = ipAddr

	recordId := strconv.FormatUint(uint64(record.ID), 10)

	err := dynv6.request("PATCH", dynv6Endpoint+"/api/v2/zones/"+zoneId+"/records/"+recordId, record, &Dynv6Record{})

	if err != nil {
		util.Log("Failed to updated domain %s! Result: %s", domain, err)
		domain.UpdateStatus = config.UpdatedFailed
	} else {
		util.Log("Updated domain %s successfully! IP: %s", domain, ipAddr)
		domain.UpdateStatus = config.UpdatedSuccess
	}
}

// request shared request method
func (dynv6 *Dynv6) request(method string, url string, data interface{}, result interface{}) (err error) {
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
		return err
	}

	req.Header.Add("Authorization", "Bearer "+dynv6.DNS.Secret)
	req.Header.Set("Content-Type", "application/json")

	client := util.CreateHTTPClient()
	resp, err := client.Do(req)
	err = util.GetHTTPResponse(resp, err, result)
	return err
}
