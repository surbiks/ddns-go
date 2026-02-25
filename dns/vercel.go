package dns

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jeessy2/ddns-go/v6/config"
	"github.com/jeessy2/ddns-go/v6/util"
)

type Vercel struct {
	DNS     config.DNS
	Domains config.Domains
	TTL     int
}

type ListExistingRecordsResponse struct {
	Records []Record `json:"records"`
}

type Record struct {
	ID        string  `json:"id"` // record ID
	Slug      string  `json:"slug"`
	Name      string  `json:"name"`  // record name
	Type      string  `json:"type"`  // record type
	Value     string  `json:"value"` // record value
	Creator   string  `json:"creator"`
	Created   int64   `json:"created"`
	Updated   int64   `json:"updated"`
	CreatedAt int64   `json:"createdAt"`
	UpdatedAt int64   `json:"updatedAt"`
	TTL       int64   `json:"ttl"`
	Comment   *string `json:"comment,omitempty"`
}

func (v *Vercel) Init(dnsConf *config.DnsConfig, ipv4cache *util.IpCache, ipv6cache *util.IpCache) {
	v.Domains.Ipv4Cache = ipv4cache
	v.Domains.Ipv6Cache = ipv6cache
	v.DNS = dnsConf.DNS
	v.Domains.GetNewIp(dnsConf)

	// Must be greater than 60
	ttl, err := strconv.Atoi(dnsConf.TTL)
	if err != nil {
		ttl = 60
	}
	if ttl < 60 {
		ttl = 60
	}
	v.TTL = ttl
}

func (v *Vercel) AddUpdateDomainRecords() (domains config.Domains) {
	v.addUpdateDomainRecords("A")
	v.addUpdateDomainRecords("AAAA")
	return v.Domains
}

func (v *Vercel) addUpdateDomainRecords(recordType string) {
	ipAddr, domains := v.Domains.GetNewIpResult(recordType)

	if ipAddr == "" {
		return
	}

	ipAddr = strings.ToLower(ipAddr)

	var (
		records []Record
		err     error
	)
	for _, domain := range domains {
		records, err = v.listExistingRecords(domain)
		if err != nil {
			util.Log("Failed to query domain info! %s", err)
			continue
		}

		var targetRecord *Record
		for _, record := range records {
			if record.Name == domain.SubDomain {
				targetRecord = &record
				break
			}
		}

		if targetRecord == nil {
			err = v.createRecord(domain, recordType, ipAddr)
		} else {
			if strings.ToLower(targetRecord.Value) == ipAddr {
				util.Log("Your's IP %s has not changed! Domain: %s", ipAddr, domain)
				domain.UpdateStatus = config.UpdatedNothing
				continue
			} else {
				err = v.updateRecord(targetRecord, recordType, ipAddr)
			}
		}

		operation := "add"
		if targetRecord != nil {
			operation = "update"
		}
		if err == nil {
			util.Log(operation+"DNS record %s success! IP: %s", domain, ipAddr)
			domain.UpdateStatus = config.UpdatedSuccess
		} else {
			util.Log(operation+"DNS record %s failed! Exception: %s", domain, err)
			domain.UpdateStatus = config.UpdatedFailed
		}
	}
}

func (v *Vercel) listExistingRecords(domain *config.Domain) (records []Record, err error) {
	var result ListExistingRecordsResponse
	err = v.request(http.MethodGet, "https://api.vercel.com/v4/domains/"+domain.DomainName+"/records", nil, &result)
	if err != nil {
		return
	}
	records = result.Records
	return
}

func (v *Vercel) createRecord(domain *config.Domain, recordType string, recordValue string) (err error) {
	err = v.request(http.MethodPost, "https://api.vercel.com/v2/domains/"+domain.DomainName+"/records", map[string]interface{}{
		"name":    domain.SubDomain,
		"type":    recordType,
		"value":   recordValue,
		"ttl":     v.TTL,
		"comment": "Created by ddns-go",
	}, nil)
	return
}

func (v *Vercel) updateRecord(record *Record, recordType string, recordValue string) (err error) {
	err = v.request(http.MethodPatch, "https://api.vercel.com/v1/domains/records/"+record.ID, map[string]interface{}{
		"type":  recordType,
		"value": recordValue,
		"ttl":   v.TTL,
	}, nil)
	return
}

func (v *Vercel) request(method, api string, data, result interface{}) (err error) {
	var payload []byte
	if data != nil {
		payload, _ = json.Marshal(data)
	}

	// ExtParam (TeamId) add parameters
	if v.DNS.ExtParam != "" {
		if strings.Contains(api, "?") {
			api = api + "&teamId=" + v.DNS.ExtParam
		} else {
			api = api + "?teamId=" + v.DNS.ExtParam
		}
	}

	req, err := http.NewRequest(
		method,
		api,
		bytes.NewBuffer(payload),
	)
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+v.DNS.Secret)
	req.Header.Set("Content-Type", "application/json")

	client := util.CreateHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("Vercel API returned status code %d", resp.StatusCode)
	}
	if result != nil {
		err = util.GetHTTPResponse(resp, err, result)
	}
	return
}
