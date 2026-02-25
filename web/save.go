package web

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/jeessy2/ddns-go/v6/config"
	"github.com/jeessy2/ddns-go/v6/dns"
	"github.com/jeessy2/ddns-go/v6/util"
)

// Save save
func Save(writer http.ResponseWriter, request *http.Request) {
	result := checkAndSave(request)
	dnsConfJsonStr := "[]"
	if result == "ok" {
		conf, _ := config.GetConfigCached()
		dnsConfJsonStr = getDnsConfStr(conf.DnsConf)
	}
	byt, _ := json.Marshal(map[string]string{"result": result, "dnsConf": dnsConfJsonStr})

	writer.Write(byt)
}

func checkAndSave(request *http.Request) string {
	conf, _ := config.GetConfigCached()

	// read JSON data from request
	var data struct {
		Username           string       `json:"Username"`
		Password           string       `json:"Password"`
		NotAllowWanAccess  bool         `json:"NotAllowWanAccess"`
		WebhookURL         string       `json:"WebhookURL"`
		WebhookRequestBody string       `json:"WebhookRequestBody"`
		WebhookHeaders     string       `json:"WebhookHeaders"`
		DnsConf            []dnsConf4JS `json:"DnsConf"`
	}

	// parserequest JSON data
	err := json.NewDecoder(request.Body).Decode(&data)
	if err != nil {
		return util.LogStr("Data parsing failed, please refresh the page and try again")
	}
	usernameNew := strings.TrimSpace(data.Username)
	passwordNew := data.Password

	//
	accept := request.Header.Get("Accept-Language")
	conf.Lang = util.InitLogLang(accept)

	conf.NotAllowWanAccess = data.NotAllowWanAccess
	conf.WebhookURL = strings.TrimSpace(data.WebhookURL)
	conf.WebhookRequestBody = strings.TrimSpace(data.WebhookRequestBody)
	conf.WebhookHeaders = strings.TrimSpace(data.WebhookHeaders)

	// check , /
	conf.Username = usernameNew
	if passwordNew != "" {
		hashedPwd, err := conf.CheckPassword(passwordNew)
		if err != nil {
			return err.Error()
		}
		conf.Password = hashedPwd
	}

	// username/password cannot be empty
	if conf.Username == "" || conf.Password == "" {
		return util.LogStr("Username/Password is required")
	}

	dnsConfFromJS := data.DnsConf
	var dnsConfArray []config.DnsConfig
	empty := dnsConf4JS{}
	for k, v := range dnsConfFromJS {
		if v == empty {
			continue
		}
		dnsConf := config.DnsConfig{Name: v.Name, TTL: v.TTL}
		// config
		dnsConf.DNS.Name = v.DnsName
		dnsConf.DNS.ID = strings.TrimSpace(v.DnsID)
		dnsConf.DNS.Secret = strings.TrimSpace(v.DnsSecret)
		dnsConf.DNS.ExtParam = strings.TrimSpace(v.DnsExtParam)

		if v.Ipv4Domains == "" && v.Ipv6Domains == "" {
			util.Log("The %s config does not fill in the domain", util.Ordinal(k+1, conf.Lang))
		}

		dnsConf.Ipv4.Enable = v.Ipv4Enable
		dnsConf.Ipv4.GetType = v.Ipv4GetType
		dnsConf.Ipv4.URL = strings.TrimSpace(v.Ipv4Url)
		dnsConf.Ipv4.NetInterface = v.Ipv4NetInterface
		dnsConf.Ipv4.Cmd = strings.TrimSpace(v.Ipv4Cmd)
		dnsConf.Ipv4.Domains = util.SplitLines(v.Ipv4Domains)

		dnsConf.Ipv6.Enable = v.Ipv6Enable
		dnsConf.Ipv6.GetType = v.Ipv6GetType
		dnsConf.Ipv6.URL = strings.TrimSpace(v.Ipv6Url)
		dnsConf.Ipv6.NetInterface = v.Ipv6NetInterface
		dnsConf.Ipv6.Cmd = strings.TrimSpace(v.Ipv6Cmd)
		dnsConf.Ipv6.Ipv6Reg = strings.TrimSpace(v.Ipv6Reg)
		dnsConf.Ipv6.Domains = util.SplitLines(v.Ipv6Domains)

		if k < len(conf.DnsConf) {
			c := &conf.DnsConf[k]
			idHide, secretHide := getHideIDSecret(c)
			if dnsConf.DNS.ID == idHide {
				dnsConf.DNS.ID = c.DNS.ID
			}
			if dnsConf.DNS.Secret == secretHide {
				dnsConf.DNS.Secret = c.DNS.Secret
			}
		}

		dnsConfArray = append(dnsConfArray, dnsConf)
	}
	conf.DnsConf = dnsConfArray

	// save
	err = conf.SaveConfig()

	//
	util.ForceCompareGlobal = true
	go dns.RunOnce()

	//
	if err != nil {
		return err.Error()
	}
	return "ok"
}
