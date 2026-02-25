package dns

import (
	"encoding/xml"
	"io"
	"net/http"
	"strings"

	"github.com/jeessy2/ddns-go/v6/config"
	"github.com/jeessy2/ddns-go/v6/util"
)

const (
	nameSiloListRecordEndpoint   = "https://www.namesilo.com/api/dnsListRecords?version=1&type=xml&key=#{password}&domain=#{domain}"
	nameSiloAddRecordEndpoint    = "https://www.namesilo.com/api/dnsAddRecord?version=1&type=xml&key=#{password}&domain=#{domain}&rrhost=#{host}&rrtype=#{recordType}&rrvalue=#{ip}&rrttl=3600"
	nameSiloUpdateRecordEndpoint = "https://www.namesilo.com/api/dnsUpdateRecord?version=1&type=xml&key=#{password}&domain=#{domain}&rrhost=#{host}&rrid=#{recordID}&rrvalue=#{ip}&rrttl=3600"
)

// NameSilo Domain
type NameSilo struct {
	DNS      config.DNS
	Domains  config.Domains
	lastIpv4 string
	lastIpv6 string
}

// NameSiloResp modifyDNS recordresult
type NameSiloResp struct {
	XMLName xml.Name      `xml:"namesilo"`
	Request Request       `xml:"request"`
	Reply   ReplyResponse `xml:"reply"`
}

type ReplyResponse struct {
	Code     int    `xml:"code"`
	Detail   string `xml:"detail"`
	RecordID string `xml:"record_id"`
}

type NameSiloDNSListRecordResp struct {
	XMLName xml.Name `xml:"namesilo"`
	Request Request  `xml:"request"`
	Reply   Reply    `xml:"reply"`
}

type Request struct {
	Operation string `xml:"operation"`
	IP        string `xml:"ip"`
}

type Reply struct {
	Code          int              `xml:"code"`
	Detail        string           `xml:"detail"`
	ResourceItems []ResourceRecord `xml:"resource_record"`
}

type ResourceRecord struct {
	RecordID string `xml:"record_id"`
	Type     string `xml:"type"`
	Host     string `xml:"host"`
	Value    string `xml:"value"`
	TTL      int    `xml:"ttl"`
	Distance int    `xml:"distance"`
}

// Init
func (ns *NameSilo) Init(dnsConf *config.DnsConfig, ipv4cache *util.IpCache, ipv6cache *util.IpCache) {
	ns.Domains.Ipv4Cache = ipv4cache
	ns.Domains.Ipv6Cache = ipv6cache
	ns.lastIpv4 = ipv4cache.Addr
	ns.lastIpv6 = ipv6cache.Addr

	ns.DNS = dnsConf.DNS
	ns.Domains.GetNewIp(dnsConf)
}

// AddUpdateDomainRecords add or update IPv4/IPv6 records
func (ns *NameSilo) AddUpdateDomainRecords() config.Domains {
	ns.addUpdateDomainRecords("A")
	ns.addUpdateDomainRecords("AAAA")
	return ns.Domains
}

func (ns *NameSilo) addUpdateDomainRecords(recordType string) {
	ipAddr, domains := ns.Domains.GetNewIpResult(recordType)

	if ipAddr == "" {
		return
	}

	for _, domain := range domains {

		if domain.SubDomain == "" {
			domain.SubDomain = "@"
		}

		// DNSrecord list domain id id modify ID add
		records, err := ns.listRecords(domain)
		if err != nil {
			util.Log("Failed to query domain info! %s", err)
			domain.UpdateStatus = config.UpdatedFailed
			return
		}
		items := records.Reply.ResourceItems
		record := findResourceRecord(items, recordType, domain.SubDomain)
		var isAdd bool
		var recordID string
		if record == nil {
			isAdd = true
		} else {
			recordID = record.RecordID
			if record.Value == ipAddr {
				util.Log("Your's IP %s has not changed! Domain: %s", ipAddr, domain)
				continue
			}
		}
		ns.modify(domain, recordID, recordType, ipAddr, isAdd)
	}
}

// modify
func (ns *NameSilo) modify(domain *config.Domain, recordID, recordType, ipAddr string, isAdd bool) {
	var err error
	var result string
	var requestType string
	if isAdd {
		requestType = "add"
		result, err = ns.request(ipAddr, domain, "", recordType, nameSiloAddRecordEndpoint)
	} else {
		requestType = "update"
		result, err = ns.request(ipAddr, domain, recordID, "", nameSiloUpdateRecordEndpoint)
	}
	if err != nil {
		util.Log("Exception: %s", err)
		domain.UpdateStatus = config.UpdatedFailed
		return
	}
	var resp NameSiloResp
	xml.Unmarshal([]byte(result), &resp)
	if resp.Reply.Code == 300 {
		util.Log(requestType+"DNS record %s success! IP: %s\n", domain, ipAddr)
		domain.UpdateStatus = config.UpdatedSuccess
	} else {
		util.Log(requestType+"DNS record %s failed! Exception: %s", domain, resp.Reply.Detail)
		domain.UpdateStatus = config.UpdatedFailed
	}
}

func (ns *NameSilo) listRecords(domain *config.Domain) (*NameSiloDNSListRecordResp, error) {
	result, err := ns.request("", domain, "", "", nameSiloListRecordEndpoint)
	if err != nil {
		return nil, err
	}

	var resp NameSiloDNSListRecordResp
	if err = xml.Unmarshal([]byte(result), &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// request shared request method
func (ns *NameSilo) request(ipAddr string, domain *config.Domain, recordID, recordType, url string) (result string, err error) {
	url = strings.NewReplacer(
		"#{host}", domain.SubDomain,
		"#{domain}", domain.DomainName,
		"#{password}", ns.DNS.Secret,
		"#{recordID}", recordID,
		"#{recordType}", recordType,
		"#{ip}", ipAddr,
	).Replace(url)
	req, err := http.NewRequest(
		http.MethodGet,
		url,
		http.NoBody,
	)

	if err != nil {
		return
	}

	client := util.CreateHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return
	}

	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	result = string(data)
	return
}

func findResourceRecord(data []ResourceRecord, recordType, domain string) *ResourceRecord {
	for i := 0; i < len(data); i++ {
		if data[i].Host == domain && data[i].Type == recordType {
			return &data[i]
		}
	}
	return nil
}
