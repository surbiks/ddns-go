package dns

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jeessy2/ddns-go/v6/config"
	"github.com/jeessy2/ddns-go/v6/util"
)

const (
	aliesaEndpoint string = "https://esa.cn-hangzhou.aliyuncs.com/"
)

// Aliesa Aliesa
type Aliesa struct {
	DNS     config.DNS
	Domains config.Domains
	TTL     string

	siteCache   map[string]AliesaSite
	domainCache config.DomainTuples
}

// AliesaSiteResp result
type AliesaSiteResp struct {
	TotalCount int
	Sites      []AliesaSite
}

// AliesaSites
type AliesaSite struct {
	SiteId     int64
	SiteName   string
	AccessType string
}

// AliesaRecordResp record response
type AliesaRecordResp struct {
	TotalCount int
	Records    []AliesaRecord
}

// AliesaRecord record
type AliesaRecord struct {
	RecordId   int64
	RecordName string
	Data       struct {
		Value string
	}
}

// AliesaResp modify/add result
type AliesaResp struct {
	OriginPoolId int64 `json:"Id"`
	RecordID     int64
	RequestID    string
}

// Init
func (ali *Aliesa) Init(dnsConf *config.DnsConfig, ipv4cache *util.IpCache, ipv6cache *util.IpCache) {
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
func (ali *Aliesa) AddUpdateDomainRecords() config.Domains {
	ali.siteCache = make(map[string]AliesaSite)
	ali.domainCache = ali.Domains.GetAllNewIpResult("A/AAAA")
	ali.addUpdateDomainRecords("A")
	ali.addUpdateDomainRecords("AAAA")
	ali.addUpdateDomainRecords("A/AAAA")
	return ali.Domains
}

func (ali *Aliesa) addUpdateDomainRecords(recordType string) {
	for _, domain := range ali.domainCache {
		if domain.RecordType != recordType {
			continue
		}

		// get site
		siteSelected, err := ali.getSite(domain)
		if err != nil {
			util.Log("Failed to query domain info! %s", err)
			domain.SetUpdateStatus(config.UpdatedFailed)
			return
		}
		if siteSelected.SiteId == 0 {
			util.Log("Root domain not found in DNS provider: %s", domain.Primary.DomainName)
			domain.SetUpdateStatus(config.UpdatedFailed)
			return
		}

		// handle address
		poolId, origins, err := ali.getOriginPool(siteSelected, domain)
		if err != nil {
			util.Log("Failed to query domain info! %s", err)
			domain.SetUpdateStatus(config.UpdatedFailed)
			return
		}
		// TODO ip
		if len(origins) != 0 {
			ali.updateOriginPool(siteSelected, domain, poolId, origins)
			return
		}

		// get records
		recordSelected, err := ali.getRecord(siteSelected, domain, "A/AAAA")
		if err != nil {
			util.Log("Failed to query domain info! %s", err)
			domain.SetUpdateStatus(config.UpdatedFailed)
			return
		}
		if recordSelected.RecordId != 0 {
			// update
			ali.modify(recordSelected, domain, "A/AAAA")
		} else {
			// create
			ali.create(siteSelected, domain, "A/AAAA")
		}
	}
}

// create
// https://help.aliyun.com/zh/edge-security-acceleration/esa/api-esa-2024-09-10-createrecord
func (ali *Aliesa) create(site AliesaSite, domainTuple *config.DomainTuple, recordType string) {
	domain := domainTuple.Primary
	ipAddr := domainTuple.GetIpAddrPool(",")

	params := domain.GetCustomParams()
	params.Set("Action", "CreateRecord")
	params.Set("SiteId", strconv.FormatInt(site.SiteId, 10))
	params.Set("RecordName", domain.String())

	params.Set("Type", recordType)
	params.Set("Data", `{"Value":"`+ipAddr+`"}`)
	params.Set("Ttl", ali.TTL)

	// compatible CNAME
	if site.AccessType == "CNAME" && !params.Has("Proxied") {
		params.Set("Proxied", "true")
	}
	if params.Has("Proxied") && !params.Has("BizName") {
		params.Set("BizName", "web")
	}

	var result AliesaResp
	err := ali.request(http.MethodPost, params, &result)

	if err != nil {
		util.Log("Failed to add domain %s! Result: %s", domain, err)
		domainTuple.SetUpdateStatus(config.UpdatedFailed)
		return
	}

	if result.RecordID != 0 {
		util.Log("Added domain %s successfully! IP: %s", domain, ipAddr)
		domainTuple.SetUpdateStatus(config.UpdatedSuccess)
	} else {
		util.Log("Failed to add domain %s! Result: %s", domain, "returned empty RecordId")
		domainTuple.SetUpdateStatus(config.UpdatedFailed)
	}
}

// modify
// https://help.aliyun.com/zh/edge-security-acceleration/esa/api-esa-2024-09-10-updaterecord
func (ali *Aliesa) modify(record AliesaRecord, domainTuple *config.DomainTuple, recordType string) {
	domain := domainTuple.Primary
	ipAddr := domainTuple.GetIpAddrPool(",")
	// skip if unchanged
	if record.Data.Value == ipAddr {
		util.Log("Your's IP %s has not changed! Domain: %s", ipAddr, domain)
		return
	}

	params := domain.GetCustomParams()
	params.Set("Action", "UpdateRecord")
	params.Set("RecordId", strconv.FormatInt(record.RecordId, 10))

	params.Set("Type", recordType)
	params.Set("Data", `{"Value":"`+ipAddr+`"}`)
	params.Set("Ttl", ali.TTL)

	var result AliesaResp
	err := ali.request(http.MethodPost, params, &result)

	if err != nil {
		util.Log("Failed to updated domain %s! Result: %s", domain, err)
		domainTuple.SetUpdateStatus(config.UpdatedFailed)
		return
	}

	// check result.RecordID updatesuccess 0
	util.Log("Updated domain %s successfully! IP: %s", domain, ipAddr)
	domainTuple.SetUpdateStatus(config.UpdatedSuccess)
}

// get current domain info
// https://help.aliyun.com/zh/edge-security-acceleration/esa/api-esa-2024-09-10-listrecords
func (ali *Aliesa) getRecord(site AliesaSite, domainTuple *config.DomainTuple, recordType string) (result AliesaRecord, err error) {
	domain := domainTuple.Primary
	var recordResp AliesaRecordResp

	params := url.Values{}
	params.Set("Action", "ListRecords")
	params.Set("SiteId", strconv.FormatInt(site.SiteId, 10))
	params.Set("RecordName", domain.String())
	params.Set("Type", recordType)
	err = ali.request(http.MethodGet, params, &recordResp)

	// recordResp.TotalCount == 0
	if len(recordResp.Records) == 0 {
		return
	}

	// RecordId
	recordId := domain.GetCustomParams().Get("RecordId")
	if recordId != "" {
		for i := 0; i < len(recordResp.Records); i++ {
			if strconv.FormatInt(recordResp.Records[i].RecordId, 10) == recordId {
				return recordResp.Records[i], nil
			}
		}
	}
	return recordResp.Records[0], nil
}

// get domain site info
// https://help.aliyun.com/zh/edge-security-acceleration/esa/api-esa-2024-09-10-listsites
func (ali *Aliesa) getSite(domainTuple *config.DomainTuple) (result AliesaSite, err error) {
	domain := domainTuple.Primary
	if site, ok := ali.siteCache[domain.DomainName]; ok {
		return site, nil
	}

	// parse parameters SiteId api GetSite
	siteIdStr := domain.GetCustomParams().Get("SiteId")
	if siteId, _ := strconv.ParseInt(siteIdStr, 10, 64); siteId != 0 {
		// compatible CNAME
		result.AccessType = "CNAME"
		result.SiteName = domain.DomainName
		result.SiteId = siteId
		return
	}

	var siteResp AliesaSiteResp
	params := url.Values{}
	params.Set("Action", "ListSites")
	params.Set("SiteName", domain.DomainName)
	err = ali.request(http.MethodGet, params, &siteResp)

	if err != nil {
		return
	}

	// siteResp.TotalCount == 0
	if len(siteResp.Sites) == 0 {
		return
	}

	result = siteResp.Sites[0]
	ali.siteCache[domain.DomainName] = result
	return
}

// getOriginPool get origin pool
// https://help.aliyun.com/zh/edge-security-acceleration/esa/api-esa-2024-09-10-listoriginpools
func (ali *Aliesa) getOriginPool(site AliesaSite, domainTuple *config.DomainTuple) (id int64, origins []map[string]interface{}, err error) {
	name, found := strings.CutSuffix(domainTuple.Primary.SubDomain, ".origin-pool")
	if !found {
		return
	}

	params := url.Values{}
	params.Set("Action", "ListOriginPools")
	params.Set("SiteId", strconv.FormatInt(site.SiteId, 10))
	params.Set("Name", name)
	params.Set("MatchType", "exact")

	result := struct {
		TotalCount  int
		OriginPools []struct {
			Id      int64
			Origins []map[string]interface{}
		}
	}{}

	err = ali.request(http.MethodGet, params, &result)
	if err == nil && len(result.OriginPools) > 0 {
		pool := result.OriginPools[0]
		id = pool.Id
		origins = pool.Origins
	}
	return
}

// updateOriginPool update address
// https://help.aliyun.com/zh/edge-security-acceleration/esa/api-esa-2024-09-10-updateoriginpool
func (ali *Aliesa) updateOriginPool(site AliesaSite, domainTuple *config.DomainTuple, id int64, origins []map[string]interface{}) {
	needUpdate := false
	count := len(domainTuple.Domains)
	for _, origin := range origins {
		// address address Domain
		for i, d := range domainTuple.Domains {
			name := d.GetCustomParams().Get("Name")
			if origin["Name"] != name {
				continue
			}
			// skip if unchanged
			address := domainTuple.IpAddrs[i]
			if origin["Address"] != address {
				origin["Address"] = address
				needUpdate = true
			}
			count--
			break
		}
	}

	domain := domainTuple.Primary
	ipAddr := domainTuple.GetIpAddrPool(",")
	if count > 0 {
		// add address
		util.Log("Failed to updated domain %s! Result: %s", domain, "adding new origin addresses is not supported")
		domainTuple.SetUpdateStatus(config.UpdatedFailed)
		return
	}
	if !needUpdate {
		util.Log("Your's IP %s has not changed! Domain: %s", ipAddr, domain)
		return
	}

	originsData, _ := json.Marshal(origins)
	params := url.Values{}
	params.Set("Action", "UpdateOriginPool")
	params.Set("SiteId", strconv.FormatInt(site.SiteId, 10))
	params.Set("Id", strconv.FormatInt(id, 10))
	params.Set("Origins", string(originsData))

	result := AliesaResp{}
	err := ali.request(http.MethodPost, params, &result)

	if err != nil {
		util.Log("Failed to updated domain %s! Result: %s", domain, err)
		domainTuple.SetUpdateStatus(config.UpdatedFailed)
		return
	}

	if result.OriginPoolId != 0 {
		util.Log("Updated domain %s successfully! IP: %s", domain, ipAddr)
		domainTuple.SetUpdateStatus(config.UpdatedSuccess)
	} else {
		util.Log("Failed to updated domain %s! Result: %s", domain, "returned empty OriginPool ID")
		domainTuple.SetUpdateStatus(config.UpdatedFailed)
	}
}

// request shared request method
func (ali *Aliesa) request(method string, params url.Values, result interface{}) (err error) {
	util.AliyunSigner(ali.DNS.ID, ali.DNS.Secret, &params, method, "2024-09-10")

	req, err := http.NewRequest(
		method,
		aliesaEndpoint,
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
