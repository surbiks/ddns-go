package dns

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jeessy2/ddns-go/v6/config"
	"github.com/jeessy2/ddns-go/v6/util"
)

// Eranet DNS
type Eranet struct {
	DNS     config.DNS
	Domains config.Domains
	TTL     string
}

type EranetRecord struct {
	ID     int `json:"id"`
	Domain string
	Host   string
	Type   string
	Value  string
	State  int
	// Name    string
	// Enabled string
}

type EranetRecordListResp struct {
	EranetBaseResult
	Data []EranetRecord
}

type EranetBaseResult struct {
	RequestId string `json:"RequestId"`
	Id        int    `json:"Id"`
	Error     string `json:"error"`
}

// Init
func (eranet *Eranet) Init(dnsConf *config.DnsConfig, ipv4cache *util.IpCache, ipv6cache *util.IpCache) {
	eranet.Domains.Ipv4Cache = ipv4cache
	eranet.Domains.Ipv6Cache = ipv6cache
	eranet.DNS = dnsConf.DNS
	eranet.Domains.GetNewIp(dnsConf)
	if dnsConf.TTL == "" {
		// default600s
		eranet.TTL = "600"
	} else {
		eranet.TTL = dnsConf.TTL
	}
}

// AddUpdateDomainRecords add or update IPv4/IPv6 records
func (eranet *Eranet) AddUpdateDomainRecords() config.Domains {
	eranet.addUpdateDomainRecords("A")
	eranet.addUpdateDomainRecords("AAAA")
	return eranet.Domains
}

func (eranet *Eranet) addUpdateDomainRecords(recordType string) {
	ipAddr, domains := eranet.Domains.GetNewIpResult(recordType)

	if ipAddr == "" {
		return
	}

	for _, domain := range domains {
		result, err := eranet.getRecordList(domain, recordType)
		if err != nil {
			util.Log("Failed to query domain info! %s", err)
			domain.UpdateStatus = config.UpdatedFailed
			return
		}

		if len(result.Data) > 0 {
			// first by default
			recordSelected := result.Data[0]
			params := domain.GetCustomParams()
			if params.Has("Id") {
				for i := 0; i < len(result.Data); i++ {
					if strconv.Itoa(result.Data[i].ID) == params.Get("Id") {
						recordSelected = result.Data[i]
					}
				}
			}
			// update
			eranet.modify(recordSelected, domain, recordType, ipAddr)
		} else {
			// add
			eranet.create(domain, recordType, ipAddr)
		}
	}
}

// create createDNSrecord
func (eranet *Eranet) create(domain *config.Domain, recordType string, ipAddr string) {
	param := map[string]string{
		"Domain": domain.DomainName,
		"Host":   domain.GetSubDomain(),
		"Type":   recordType,
		"Value":  ipAddr,
		"Ttl":    eranet.TTL,
	}
	res, err := eranet.request("/api/Dns/AddDomainRecord", param, "GET")
	if err != nil {
		util.Log("Failed to add domain %s! Result: %s", domain, err.Error())
		domain.UpdateStatus = config.UpdatedFailed
	}
	var result NowcnBaseResult
	err = json.Unmarshal(res, &result)
	if err != nil {
		util.Log("Failed to add domain %s! Result: %s", domain, err.Error())
		domain.UpdateStatus = config.UpdatedFailed
	}
	if result.Error != "" {
		util.Log("Failed to add domain %s! Result: %s", domain, result.Error)
		domain.UpdateStatus = config.UpdatedFailed
	} else {
		domain.UpdateStatus = config.UpdatedSuccess
	}
}

// modify modifyDNSrecord
func (eranet *Eranet) modify(record EranetRecord, domain *config.Domain, recordType string, ipAddr string) {
	// skip if unchanged
	if record.Value == ipAddr {
		util.Log("Your's IP %s has not changed! Domain: %s", ipAddr, domain)
		return
	}
	param := map[string]string{
		"Id":     strconv.Itoa(record.ID),
		"Domain": domain.DomainName,
		"Host":   domain.GetSubDomain(),
		"Type":   recordType,
		"Value":  ipAddr,
		"Ttl":    eranet.TTL,
	}
	res, err := eranet.request("/api/Dns/UpdateDomainRecord", param, "GET")
	if err != nil {
		util.Log("Failed to updated domain %s! Result: %s", domain, err.Error())
		domain.UpdateStatus = config.UpdatedFailed
	}
	var result NowcnBaseResult
	err = json.Unmarshal(res, &result)
	if err != nil {
		util.Log("Failed to updated domain %s! Result: %s", domain, err.Error())
		domain.UpdateStatus = config.UpdatedFailed
	}
	if result.Error != "" {
		util.Log("Failed to updated domain %s! Result: %s", domain, result.Error)
		domain.UpdateStatus = config.UpdatedFailed
	} else {
		util.Log("Updated domain %s successfully! IP: %s", domain, ipAddr)
		domain.UpdateStatus = config.UpdatedSuccess
	}
}

// getRecordList get domain record list
func (eranet *Eranet) getRecordList(domain *config.Domain, typ string) (result EranetRecordListResp, err error) {
	param := map[string]string{
		"Domain": domain.DomainName,
		"Type":   typ,
		"Host":   domain.GetSubDomain(),
	}
	res, err := eranet.request("/api/Dns/DescribeRecordIndex", param, "GET")
	err = json.Unmarshal(res, &result)
	return
}

func (eranet *Eranet) queryParams(param map[string]any) string {
	var queryParams []string
	for key, value := range param {
		// URL @
		encodedKey := url.QueryEscape(key)
		valueStr := fmt.Sprintf("%v", value)
		// @
		encodedValue := strings.ReplaceAll(url.QueryEscape(valueStr), "%40", "@")
		encodedValue = strings.ReplaceAll(encodedValue, "%3A", ":")
		queryParams = append(queryParams, encodedKey+"="+encodedValue)
	}
	return strings.Join(queryParams, "&")
}

func (t *Eranet) sign(params map[string]string, method string) (string, error) {
	// add parameters
	params["AccessKeyID"] = t.DNS.ID
	params["SignatureMethod"] = "HMAC-SHA1"
	params["SignatureNonce"] = fmt.Sprintf("%d", time.Now().UnixNano())
	params["Timestamp"] = time.Now().UTC().Format("2006-01-02T15:04:05Z")

	// 1. sort parameters(alphabetical order)
	var keys []string
	for k := range params {
		if k != "Signature" { // exclude Signature parameter
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	// 2. request
	var canonicalizedQuery []string
	for _, k := range keys {
		// URL-encode parameter names and values
		encodedKey := util.PercentEncode(k)
		encodedValue := util.PercentEncode(params[k])
		canonicalizedQuery = append(canonicalizedQuery, encodedKey+"="+encodedValue)
	}
	canonicalizedQueryString := strings.Join(canonicalizedQuery, "&")

	// 3. build string to sign
	stringToSign := method + "&" + util.PercentEncode("/") + "&" + util.PercentEncode(canonicalizedQueryString)

	// 4. calculate HMAC-SHA1 signature
	key := t.DNS.Secret + "&"
	h := hmac.New(sha1.New, []byte(key))
	h.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	// 5. add parameters
	params["Signature"] = signature

	// 6. rebuild final query string( )
	keys = append(keys, "Signature")
	sort.Strings(keys)
	var finalQuery []string
	for _, k := range keys {
		encodedKey := util.PercentEncode(k)
		encodedValue := util.PercentEncode(params[k])
		finalQuery = append(finalQuery, encodedKey+"="+encodedValue)
	}

	return strings.Join(finalQuery, "&"), nil
}

func (t *Eranet) request(apiPath string, params map[string]string, method string) ([]byte, error) {
	// generate signature
	queryString, err := t.sign(params, method)
	if err != nil {
		return nil, fmt.Errorf("Failed to generate signature: %v", err)
	}

	// build full URL
	baseURL := "https://www.eranet.com"
	fullURL := baseURL + apiPath + "?" + queryString

	// createHTTPrequest
	req, err := http.NewRequest(method, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to create request: %v", err)
	}

	// set request headers
	req.Header.Set("Accept", "application/json")

	// send request
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Failed to read response: %v", err)
	}

	// checkHTTPstatus code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed, status code: %d, response: %s", resp.StatusCode, string(body))
	}

	return body, nil
}
