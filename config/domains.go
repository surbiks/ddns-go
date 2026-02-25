package config

import (
	"net/url"
	"strings"

	"github.com/jeessy2/ddns-go/v6/util"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"
)

// Domains Ipv4/Ipv6 domains
type Domains struct {
	Ipv4Addr    string
	Ipv4Cache   *util.IpCache
	Ipv4Domains []*Domain
	Ipv6Addr    string
	Ipv6Cache   *util.IpCache
	Ipv6Domains []*Domain
}

// Domain domain
type Domain struct {
	// DomainName domain
	DomainName string
	// SubDomain domain
	SubDomain    string
	CustomParams string
	UpdateStatus updateStatusType // updatestatus
}

// DomainTuples domain key: Domain.String()
type DomainTuples map[string]*DomainTuple

// DomainTuple domain
type DomainTuple struct {
	RecordType string
	// Primary domain Domains[-1] = Primary
	Primary  *Domain
	Domains  []*Domain
	IpAddrs  []string
	Ipv4Addr string
	Ipv6Addr string
}

// nontransitionalLookup implements the nontransitional processing as specified in
// Unicode Technical Standard 46 with almost all checkings off to maximize user freedom.
//
// Copied from: https://github.com/cloudflare/cloudflare-go/blob/v0.97.0/dns.go#L95
var nontransitionalLookup = idna.New(
	idna.MapForLookup(),
	idna.StrictDomainName(false),
	idna.ValidateLabels(false),
)

func (d Domain) String() string {
	if d.SubDomain != "" {
		return d.SubDomain + "." + d.DomainName
	}
	return d.DomainName
}

// GetFullDomain get domain
func (d Domain) GetFullDomain() string {
	if d.SubDomain != "" {
		return d.SubDomain + "." + d.DomainName
	}
	return "@" + "." + d.DomainName
}

// GetSubDomain get domain @
// / /dnspod/GoDaddy/namecheap
func (d Domain) GetSubDomain() string {
	if d.SubDomain != "" {
		return d.SubDomain
	}
	return "@"
}

// GetCustomParams not be nil
func (d Domain) GetCustomParams() url.Values {
	if d.CustomParams != "" {
		q, err := url.ParseQuery(d.CustomParams)
		if err == nil {
			return q
		}
	}
	return url.Values{}
}

// ToASCII converts [Domain] to its ASCII form,
// using non-transitional process specified in UTS 46.
//
// Note: conversion errors are silently discarded and partial conversion
// results are used.
func (d Domain) ToASCII() string {
	name, _ := nontransitionalLookup.ToASCII(d.String())
	return name
}

// GetNewIp / / get ip domain
func (domains *Domains) GetNewIp(dnsConf *DnsConfig) {
	domains.Ipv4Domains = checkParseDomains(dnsConf.Ipv4.Domains)
	domains.Ipv6Domains = checkParseDomains(dnsConf.Ipv6.Domains)

	// IPv4
	if dnsConf.Ipv4.Enable && len(domains.Ipv4Domains) > 0 {
		ipv4Addr := dnsConf.GetIpv4Addr()
		if ipv4Addr != "" {
			domains.Ipv4Addr = ipv4Addr
			domains.Ipv4Cache.TimesFailedIP = 0
		} else {
			// IPv4 & get IP & domain & failed 3 failed
			domains.Ipv4Cache.TimesFailedIP++
			if domains.Ipv4Cache.TimesFailedIP == 3 {
				domains.Ipv4Domains[0].UpdateStatus = UpdatedFailed
			}
			util.Log("Failed to get IPv4 address, will not update")
		}
	}

	// IPv6
	if dnsConf.Ipv6.Enable && len(domains.Ipv6Domains) > 0 {
		ipv6Addr := dnsConf.GetIpv6Addr()
		if ipv6Addr != "" {
			domains.Ipv6Addr = ipv6Addr
			domains.Ipv6Cache.TimesFailedIP = 0
		} else {
			// IPv6 & get IP & domain & failed 3 failed
			domains.Ipv6Cache.TimesFailedIP++
			if domains.Ipv6Cache.TimesFailedIP == 3 {
				domains.Ipv6Domains[0].UpdateStatus = UpdatedFailed
			}
			util.Log("Failed to get IPv6 address, will not update")
		}
	}

}

// checkParseDomains parse domain
func checkParseDomains(domainArr []string) (domains []*Domain) {
	for _, domainStr := range domainArr {
		domainStr = strings.TrimSpace(domainStr)
		if domainStr == "" {
			continue
		}

		domain := &Domain{}

		// qp(queryParts) domain parameters baidu.com?q=1 => [baidu.com, q=1]
		qp := strings.Split(domainStr, "?")
		domainStr = qp[0]

		// dp(domainParts) domain qp[0] domain domain www:example.cn.eu.org => [www, example.cn.eu.org]
		dp := strings.Split(domainStr, ":")

		switch len(dp) {
		case 1: // domain
			domainName, err := publicsuffix.EffectiveTLDPlusOne(domainStr)
			if err != nil {
				util.Log("The domain %s is incorrect", domainStr)
				util.Log("Exception: %s", err)
				continue
			}
			domain.DomainName = domainName

			domainLen := len(domainStr) - len(domainName) - 1
			if domainLen > 0 {
				domain.SubDomain = domainStr[:domainLen]
			}
		case 2: // domain: domain
			sp := strings.Split(dp[1], ".")
			if len(sp) <= 1 {
				util.Log("The domain %s is incorrect", domainStr)
				continue
			}
			domain.DomainName = dp[1]
			domain.SubDomain = dp[0]
		default:
			util.Log("The domain %s is incorrect", domainStr)
			continue
		}

		// parameters
		if len(qp) == 2 {
			u, err := url.Parse("https://baidu.com?" + qp[1])
			if err != nil {
				util.Log("The domain %s resolution failed", domainStr)
				continue
			}
			domain.CustomParams = u.Query().Encode()
		}
		domains = append(domains, domain)
	}
	return
}

// GetNewIpResult getGetNewIpresult
func (domains *Domains) GetNewIpResult(recordType string) (ipAddr string, retDomains []*Domain) {
	if recordType == "AAAA" {
		if domains.Ipv6Cache.Check(domains.Ipv6Addr) {
			return domains.Ipv6Addr, domains.Ipv6Domains
		} else {
			util.Log("IPv6 has not changed, will wait %d times to compare with DNS provider", domains.Ipv6Cache.Times)
			return "", domains.Ipv6Domains
		}
	}
	// IPv4
	if domains.Ipv4Cache.Check(domains.Ipv4Addr) {
		return domains.Ipv4Addr, domains.Ipv4Domains
	} else {
		util.Log("IPv4 has not changed, will wait %d times to compare with DNS provider", domains.Ipv4Cache.Times)
		return "", domains.Ipv4Domains
	}
}

// GetAllNewIpResult getgetNewIpresult
func (domains *Domains) GetAllNewIpResult(multiRecordType string) (results DomainTuples) {
	ipv4Addr, ipv4Domains := domains.GetNewIpResult("A")
	ipv6Addr, ipv6Domains := domains.GetNewIpResult("AAAA")
	if ipv4Addr == "" && ipv6Addr == "" {
		return
	}
	cap := 0
	if ipv4Addr != "" {
		cap += len(ipv4Domains)
	}
	if ipv6Addr != "" {
		cap += len(ipv6Domains)
	}

	results = make(DomainTuples, cap)
	results.append(ipv4Addr, ipv4Domains, multiRecordType, DomainTuple{RecordType: "A", Ipv4Addr: domains.Ipv4Addr, Ipv6Addr: domains.Ipv6Addr})
	results.append(ipv6Addr, ipv6Domains, multiRecordType, DomainTuple{RecordType: "AAAA", Ipv4Addr: domains.Ipv4Addr, Ipv6Addr: domains.Ipv6Addr})
	return
}

// append adddomain domain
func (domains DomainTuples) append(ipAddr string, retDomains []*Domain, multiRecordType string, template DomainTuple) {
	if ipAddr == "" {
		return
	}

	for _, domain := range retDomains {
		domainStr := domain.String()
		if tuple, ok := domains[domainStr]; ok {
			if tuple.RecordType != template.RecordType {
				tuple.RecordType = multiRecordType
			}
			tuple.Primary = domain
			tuple.Domains = append(tuple.Domains, domain)
			tuple.IpAddrs = append(tuple.IpAddrs, ipAddr)
		} else {
			tuple := template
			domains[domainStr] = &tuple
			tuple.Primary = domain
			tuple.Domains = []*Domain{domain}
			tuple.IpAddrs = []string{ipAddr}
		}
	}
}

// SetUpdateStatus set update status
func (d *DomainTuple) SetUpdateStatus(status updateStatusType) {
	if d.Primary.UpdateStatus == status {
		return
	}

	for _, domain := range d.Domains {
		domain.UpdateStatus = status
	}
}

// GetIpAddrPool set update status
func (d *DomainTuple) GetIpAddrPool(separator string) (result string) {
	s := d.Primary.GetCustomParams().Get("IpAddrPool")
	if len(s) != 0 {
		return strings.NewReplacer(
			"{ipv4Addr}", d.Ipv4Addr,
			"{ipv6Addr}", d.Ipv6Addr,
		).Replace(s)
	}
	switch d.RecordType {
	case "A":
		return d.Ipv4Addr
	case "AAAA":
		return d.Ipv6Addr
	default:
		return d.Ipv4Addr + separator + d.Ipv6Addr
	}
}
