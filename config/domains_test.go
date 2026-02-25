package config

import "testing"

// TestToASCII test converts the name of [Domain] to its ASCII form.
//
// Copied from: https://github.com/cloudflare/cloudflare-go/blob/v0.97.0/dns_test.go#L15
func TestToASCII(t *testing.T) {
	tests := map[string]struct {
		domain   string
		expected string
	}{
		"empty": {
			"", "",
		},
		"unicode get encoded": {
			"😺.com", "xn--138h.com",
		},
		"unicode gets mapped and encoded": {
			"ÖBB.at", "xn--bb-eka.at",
		},
		"punycode stays punycode": {
			"xn--138h.com", "xn--138h.com",
		},
		"hyphens are not checked": {
			"s3--s4.com", "s3--s4.com",
		},
		"STD3 rules are not enforced": {
			"℀.com", "a/c.com",
		},
		"bidi check is disabled": {
			"englishﻋﺮﺑﻲ.com", "xn--english-gqjzfwd1j.com",
		},
		"invalid joiners are allowed": {
			"a\u200cb.com", "xn--ab-j1t.com",
		},
		"partial results are used despite errors": {
			"xn--:D.xn--.😺.com", "xn--:d..xn--138h.com",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			d := &Domain{DomainName: tt.domain}
			actual := d.ToASCII()
			if actual != tt.expected {
				t.Errorf("ToASCII() = %v, want %v", actual, tt.expected)
			}
		})
	}
}

// TestParseDomainArr test parseDomainArr
func TestParseDomainArr(t *testing.T) {
	domains := []string{"mydomain.com", "test.mydomain.com", "test2.test.mydomain.com", "mydomain.com.mydomain.com", "mydomain.com.cn",
		"test.mydomain.com.cn", "test:mydomain.com.cn",
		"test.mydomain.com?Line=oversea&RecordId=123", "test.mydomain.com.cn?Line=oversea&RecordId=123",
		"test2:test.mydomain.com?Line=oversea&RecordId=123"}
	result := []Domain{
		{DomainName: "mydomain.com", SubDomain: ""},
		{DomainName: "mydomain.com", SubDomain: "test"},
		{DomainName: "mydomain.com", SubDomain: "test2.test"},
		{DomainName: "mydomain.com", SubDomain: "mydomain.com"},
		{DomainName: "mydomain.com.cn", SubDomain: ""},
		{DomainName: "mydomain.com.cn", SubDomain: "test"},
		{DomainName: "mydomain.com.cn", SubDomain: "test"},
		{DomainName: "mydomain.com", SubDomain: "test", CustomParams: "Line=oversea&RecordId=123"},
		{DomainName: "mydomain.com.cn", SubDomain: "test", CustomParams: "Line=oversea&RecordId=123"},
		{DomainName: "test.mydomain.com", SubDomain: "test2", CustomParams: "Line=oversea&RecordId=123"},
	}

	parsedDomains := checkParseDomains(domains)
	for i := 0; i < len(parsedDomains); i++ {
		if parsedDomains[i].DomainName != result[i].DomainName ||
			parsedDomains[i].SubDomain != result[i].SubDomain ||
			parsedDomains[i].CustomParams != result[i].CustomParams {
			t.Errorf("parse %s failed:\nexpected DomainName: %s, got DomainName: %s\nexpected SubDomain: %s, got SubDomain: %s\nexpected CustomParams: %s, got CustomParams: %s",
				parsedDomains[i].String(),
				result[i].DomainName, parsedDomains[i].DomainName,
				result[i].SubDomain, parsedDomains[i].SubDomain,
				result[i].CustomParams, parsedDomains[i].CustomParams)
		}
	}

}
