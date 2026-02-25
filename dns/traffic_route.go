package dns

import (
	"encoding/json"
	"strconv"

	"github.com/jeessy2/ddns-go/v6/config"
	"github.com/jeessy2/ddns-go/v6/util"
)

// TrafficRoute DNSservice
type TrafficRoute struct {
	DNS     config.DNS
	Domains config.Domains
	TTL     int
}

// TrafficRouteMeta parserecord
type TrafficRouteMeta struct {
	ZID      int    `json:"ZID"`      // domainID
	RecordID string `json:"RecordID"` // parserecord ID
	Host     string `json:"Host"`     // record
	Type     string `json:"Type"`     // record type
	Value    string `json:"Value"`    // record value
	TTL      int    `json:"TTL"`      // TTL
	Line     string `json:"Line"`     // parse
}

// TrafficRouteResp APIresponse
type TrafficRouteResp struct {
	ResponseMetadata struct {
		RequestId string `json:"RequestId"`
		Action    string `json:"Action"`
		Version   string `json:"Version"`
		Service   string `json:"Service"`
		Region    string `json:"Region"`
		Error     struct {
			Code    string `json:"Code"`
			Message string `json:"Message"`
		} `json:"Error"`
	} `json:"ResponseMetadata"`
	Result struct {
		// domain
		Zones []struct {
			ZID         int    `json:"ZID"`
			ZoneName    string `json:"ZoneName"`
			RecordCount int    `json:"RecordCount"`
		} `json:"Zones,omitempty"`
		Total int `json:"Total,omitempty"`

		// parserecord
		Records    []TrafficRouteMeta `json:"Records,omitempty"`
		TotalCount int                `json:"TotalCount,omitempty"`

		// create/updaterecord
		RecordID string `json:"RecordID,omitempty"`
		Status   bool   `json:"Status,omitempty"`
	} `json:"Result"`
}

// TrafficRouteListZonesParams ListZones parameters
type TrafficRouteListZonesParams struct {
	Key string `json:"Key,omitempty"` // get domain(default )
}

// TrafficRouteListZonesResp
type TrafficRouteListZonesResp struct {
	ZID int `json:"ZID"` // domainID
}

func (tr *TrafficRoute) Init(dnsConf *config.DnsConfig, ipv4cache *util.IpCache, ipv6cache *util.IpCache) {
	tr.Domains.Ipv4Cache = ipv4cache
	tr.Domains.Ipv6Cache = ipv6cache
	tr.DNS = dnsConf.DNS
	tr.Domains.GetNewIp(dnsConf)
	if dnsConf.TTL == "" {
		tr.TTL = 600
	} else {
		ttl, err := strconv.Atoi(dnsConf.TTL)
		if err != nil {
			tr.TTL = 600
		} else {
			tr.TTL = ttl
		}
	}
}

// AddUpdateDomainRecords add or update IPv4/IPv6 records
func (tr *TrafficRoute) AddUpdateDomainRecords() config.Domains {
	tr.addUpdateDomainRecords("A")
	tr.addUpdateDomainRecords("AAAA")
	return tr.Domains
}

func (tr *TrafficRoute) addUpdateDomainRecords(recordType string) {
	ipAddr, domains := tr.Domains.GetNewIpResult(recordType)
	if ipAddr == "" {
		return
	}

	for _, domain := range domains {
		resp := TrafficRouteListZonesResp{}
		tr.getZID(domain, &resp)
		zoneID := resp.ZID

		var recordResp TrafficRouteResp
		tr.request(
			"GET",
			"ListRecords",
			map[string][]string{
				"ZID":        {strconv.Itoa(zoneID)},
				"Type":       {recordType},
				"Host":       {domain.GetSubDomain()},
				"SearchMode": {"exact"},
				"PageNumber": {"1"},
				"PageSize":   {"500"},
			},
			&recordResp,
		)

		found := false
		for _, record := range recordResp.Result.Records {
			if record.Type == recordType && record.Host == domain.GetSubDomain() {
				tr.modify(record, domain, ipAddr)
				found = true
				break
			}
		}

		if !found {
			tr.create(zoneID, domain, recordType, ipAddr)
		}
	}
}

// getZID getdomain ZID
func (tr *TrafficRoute) getZID(domain *config.Domain, resp *TrafficRouteListZonesResp) {
	var result TrafficRouteResp
	err := tr.request(
		"GET",
		"ListZones",
		map[string][]string{"Key": {domain.DomainName}},
		&result,
	)

	if err != nil {
		util.Log("Failed to query domain info! %s", err)
		domain.UpdateStatus = config.UpdatedFailed
		return
	}

	if len(result.Result.Zones) == 0 {
		util.Log("Domain not found in DNS provider: %s", domain.DomainName)
		domain.UpdateStatus = config.UpdatedFailed
		return
	}

	for _, zone := range result.Result.Zones {
		if zone.ZoneName == domain.DomainName {
			resp.ZID = zone.ZID
			return
		}
	}
}

// create addparserecord
func (tr *TrafficRoute) create(zoneID int, domain *config.Domain, recordType, ipAddr string) {
	record := &TrafficRouteMeta{
		ZID:   zoneID,
		Host:  domain.GetSubDomain(),
		Type:  recordType,
		Value: ipAddr,
		TTL:   tr.TTL,
		Line:  "default",
	}

	var result TrafficRouteResp
	err := tr.request(
		"POST",
		"CreateRecord",
		record,
		&result,
	)

	if err != nil {
		util.Log("Failed to add domain %s! Result: %s", domain, err)
		domain.UpdateStatus = config.UpdatedFailed
		return
	}

	if result.ResponseMetadata.Error.Code == "" {
		util.Log("Added domain %s successfully! IP: %s", domain, ipAddr)
		domain.UpdateStatus = config.UpdatedSuccess
	} else {
		util.Log("Failed to add domain %s! Result: %s", domain, result.ResponseMetadata.Error.Message)
		domain.UpdateStatus = config.UpdatedFailed
	}
}

// modify modifyparserecord
func (tr *TrafficRoute) modify(record TrafficRouteMeta, domain *config.Domain, ipAddr string) {
	if record.Value == ipAddr {
		util.Log("IP %s has not changed, domain %s", ipAddr, domain)
		domain.UpdateStatus = config.UpdatedNothing
		return
	}

	record.Value = ipAddr
	record.TTL = tr.TTL

	var result TrafficRouteResp
	err := tr.request(
		"POST",
		"UpdateRecord",
		record,
		&result,
	)

	if err != nil {
		util.Log("Failed to updated domain %s! Result: %s", domain, err)
		domain.UpdateStatus = config.UpdatedFailed
		return
	}

	if result.ResponseMetadata.Error.Code == "" {
		util.Log("Updated domain %s successfully! IP: %s", domain, ipAddr)
		domain.UpdateStatus = config.UpdatedSuccess
	} else {
		util.Log("Failed to updated domain %s! Result: %s", domain, result.ResponseMetadata.Error.Message)
		domain.UpdateStatus = config.UpdatedFailed
	}
}

// parseRequestParams parserequestparameters
func (tr *TrafficRoute) parseRequestParams(action string, data interface{}) (queryParams map[string][]string, jsonStr []byte, err error) {
	queryParams = make(map[string][]string)

	switch v := data.(type) {
	case map[string][]string:
		queryParams = v
		jsonStr = []byte{}
	case *TrafficRouteMeta:
		jsonStr, _ = json.Marshal(v)
	default:
		if data != nil {
			jsonStr, _ = json.Marshal(data)
		}
	}

	// actionhandleparameters
	switch action {
	case "ListZones":
		if len(queryParams) == 0 && len(jsonStr) > 0 {
			var params TrafficRouteListZonesParams
			if err = json.Unmarshal(jsonStr, &params); err == nil && params.Key != "" {
				queryParams["Key"] = []string{params.Key}
			}
			jsonStr = []byte{}
		}
	case "ListRecords":
		if len(queryParams) == 0 && len(jsonStr) > 0 {
			var params TrafficRouteListZonesResp
			if err = json.Unmarshal(jsonStr, &params); err == nil && params.ZID != 0 {
				queryParams["ZID"] = []string{strconv.Itoa(params.ZID)}
			}
			jsonStr = []byte{}
		}
	}

	return
}

// request shared request method
func (tr *TrafficRoute) request(method string, action string, data interface{}, result interface{}) error {
	queryParams, jsonStr, err := tr.parseRequestParams(action, data)
	if err != nil {
		return err
	}

	req, err := util.TrafficRouteSigner(method, queryParams, map[string]string{}, tr.DNS.ID, tr.DNS.Secret, action, jsonStr)
	if err != nil {
		return err
	}

	client := util.CreateHTTPClient()
	resp, err := client.Do(req)
	return util.GetHTTPResponse(resp, err, result)
}
