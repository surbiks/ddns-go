package util

import (
	"log"
	"strings"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var logLang = language.English
var logPrinter = message.NewPrinter(logLang)

func init() {

	message.SetString(language.English, "You can use '.\\ddns-go.exe -s install' to install service", "You can use '.\\ddns-go.exe -s install' to install service")
	message.SetString(language.English, "You can use 'sudo ./ddns-go -s install' to install service", "You can use 'sudo ./ddns-go -s install' to install service")
	message.SetString(language.English, "Listening on %s", "Listening on %s")
	message.SetString(language.English, "Config file has been saved to: %s", "Config file has been saved to: %s")

	message.SetString(language.English, "Your's IP %s has not changed! Domain: %s", "Your's IP %s has not changed! Domain: %s")
	message.SetString(language.English, "Added domain %s successfully! IP: %s", "Added domain %s successfully! IP: %s")
	message.SetString(language.English, "Failed to add domain %s! Result: %s", "Failed to add domain %s! Result: %s")

	message.SetString(language.English, "Updated domain %s successfully! IP: %s", "Updated domain %s successfully! IP: %s")
	message.SetString(language.English, "Failed to updated domain %s! Result: %s", "Failed to updated domain %s! Result: %s")

	message.SetString(language.English, "Your's IPv4 has not changed, %s request has not been triggered", "Your's IPv4 has not changed, %s request has not been triggered")
	message.SetString(language.English, "Your's IPv6 has not changed, %s request has not been triggered", "Your's IPv6 has not changed, %s request has not been triggered")
	message.SetString(language.English, "Namecheap does not support IPv6", "Namecheap does not support IPv6")

	message.SetString(language.English, "dynadot only supports single domain configuration, please add more configurations", "dynadot only supports single domain configuration, please add more configurations")

	// http_util
	message.SetString(language.English, "Exception: %s", "Exception: %s")
	message.SetString(language.English, "Failed to query domain info! %s", "Failed to query domain info! %s")
	message.SetString(language.English, "Response body: %s ,Response status code: %d", "Response body: %s ,Response status code: %d")
	message.SetString(language.English, "Failed to get IPv4 from %s", "Failed to get IPv4 from %s")
	message.SetString(language.English, "Failed to get IPv6 from %s", "Failed to get IPv6 from %s")
	message.SetString(language.English, "Webhook will not be triggered, only trigger once when the third failure, current failure times: %d", "Webhook will not be triggered, only trigger once when the third failure, current failure times: %d")
	message.SetString(language.English, "Root domain not found in DNS provider: %s", "Root domain not found in DNS provider: %s")

	// webhook
	message.SetString(language.English, "Webhook url is incorrect", "Webhook url is incorrect")
	message.SetString(language.English, "Webhook RequestBody JSON is invalid", "Webhook RequestBody JSON is invalid")
	message.SetString(language.English, "Successfully called Webhook! Response body: %s", "Successfully called Webhook! Response body: %s")
	message.SetString(language.English, "Failed to call Webhook! Exception: %s", "Failed to call Webhook! Exception: %s")
	message.SetString(language.English, "Webhook header is invalid: %s", "Webhook header is invalid: %s")
	message.SetString(language.English, "Please enter the Webhook url", "Please enter the Webhook url")

	// callback
	message.SetString(language.English, "Callback url is incorrect", "Callback url is incorrect")
	message.SetString(language.English, "Successfully called Callback! Domain: %s, IP: %s, Response body: %s", "Successfully called Callback! Domain: %s, IP: %s, Response body: %s")
	message.SetString(language.English, "Callback call failed, Exception: %s", "Failed to call Callback! Exception: %s")

	// save
	message.SetString(language.English, "Username/Password is required", "Username/Password is required")
	message.SetString(language.English, "Password is not secure! Try using a more complex password", "Password is not secure! Try using a more complex password")
	message.SetString(language.English, "Data parsing failed, please refresh the page and try again", "Data parsing failed, please refresh the page and try again")
	message.SetString(language.English, "The %s config does not fill in the domain", "The %s config does not fill in the domain")

	// config
	message.SetString(language.English, "Failed to get IPv4 from network card", "Failed to get IPv4 from network card")
	message.SetString(language.English, "Failed to get IPv4 from network card! Network card name: %s", "Failed to get IPv4 from network card! Network card name: %s")
	message.SetString(language.English, "Failed to get IPv4 result! Interface: %s ,Result: %s", "Failed to get IPv4 result! Interface: %s ,Result: %s")
	message.SetString(language.English, "Failed to get %s result! Command: %s, Error: %q, Exit status code: %s", "Failed to get %s result! Command: %s, Error: %q, Exit status code: %s")
	message.SetString(language.English, "Failed to get %s result! Command: %s, Stdout: %q", "Failed to get %s result! Command: %s, Stdout: %q")
	message.SetString(language.English, "Failed to get IPv6 from network card", "Failed to get IPv6 from network card")
	message.SetString(language.English, "Failed to get IPv6 from network card! Network card name: %s", "Failed to get IPv6 from network card! Network card name: %s")
	message.SetString(language.English, "Failed to get IPv6 result! Interface: %s ,Result: %s", "Failed to get IPv6 result! Interface: %s ,Result: %s")
	message.SetString(language.English, "%dth IPv6 address not found! Will use the first IPv6 address", "%dth IPv6 address not found! Will use the first IPv6 address")
	message.SetString(language.English, "IPv6 match expression %s is incorrect! Minimum start from 1", "IPv6 match expression %s is incorrect! Minimum start from 1")
	message.SetString(language.English, "IPv6 will use regular expression %s for matching", "IPv6 will use regular expression %s for matching")
	message.SetString(language.English, "Match successfully! Matched address: %s", "Match successfully! Matched address: %s")
	message.SetString(language.English, "No IPv6 address matched, will use the first address", "No IPv6 address matched, will use the first address")
	message.SetString(language.English, "Failed to get IPv4 address, will not update", "Failed to get IPv4 address, will not update")
	message.SetString(language.English, "Failed to get IPv6 address, will not update", "Failed to get IPv6 address, will not update")

	// domains
	message.SetString(language.English, "The domain %s is incorrect", "The domain %s is incorrect")
	message.SetString(language.English, "The domain %s resolution failed", "The domain %s resolution failed")
	message.SetString(language.English, "DNS resolution for domain %s was not found, and the creation failed due to the added parameter %s=%s. This update has been ignored.", "DNS resolution for domain %s was not found, and the creation failed due to the added parameter %s=%s. This update has been ignored.")
	message.SetString(language.English, "IPv6 has not changed, will wait %d times to compare with DNS provider", "IPv6 has not changed, will wait %d times to compare with DNS provider")
	message.SetString(language.English, "IPv4 has not changed, will wait %d times to compare with DNS provider", "IPv4 has not changed, will wait %d times to compare with DNS provider")

	message.SetString(language.English, "Local DNS exception! Will use %s by default, you can use -dns to customize DNS server", "Local DNS exception! Will use %s by default, you can use -dns to customize DNS server")
	message.SetString(language.English, "Waiting for network connection: %s", "Waiting for network connection: %s")
	message.SetString(language.English, "Retry after %s", "Retry after %s")
	message.SetString(language.English, "The network is connected", "The network is connected")

	// main
	message.SetString(language.English, "Port listening failed, please check if the port is occupied! %s", "Port listening failed, please check if the port is occupied! %s")
	message.SetString(language.English, "ddns-go service uninstalled successfully", "ddns-go service uninstalled successfully")
	message.SetString(language.English, "ddns-go service uninstall failed, Exception: %s", "ddns-go service uninstallation failed, Exception: %s")
	message.SetString(language.English, "Installed ddns-go service successfully! Please open the browser and configure it", "Installed ddns-go service successfully! Please open the browser and configure it")
	message.SetString(language.English, "Failed to install ddns-go service, Exception: %s", "Failed to install ddns-go service, Exception: %s")
	message.SetString(language.English, "ddns-go service has been installed, no need to install again", "ddns-go service has been installed, no need to install again")
	message.SetString(language.English, "restarted ddns-go service successfully", "restarted ddns-go service successfully")
	message.SetString(language.English, "started ddns-go service successfully", "started ddns-go service successfully")
	message.SetString(language.English, "ddns-go service is not installed, please install the service first", "ddns-go service is not installed, please install the service first")

	// webhooknotification
	message.SetString(language.English, "no changed", "no changed")
	message.SetString(language.English, "failed", "failed")
	message.SetString(language.English, "success", "success")

	// Login
	message.SetString(language.English, "%q configuration file is empty, public network access is prohibited for more than 3 hours", "%q configuration file is empty, public network access is prohibited for more than 3 hours")
	message.SetString(language.English, "%q is prohibited from accessing the public network", "%q is prohibited from accessing the public network")
	message.SetString(language.English, "%q username or password is incorrect", "%q username or password is incorrect")
	message.SetString(language.English, "%q login succeeded", "%q login successfully")
	message.SetString(language.English, "Username or password is incorrect", "Username or password is incorrect")
	message.SetString(language.English, "Too many login failures, please try again later", "Too many login failures, please try again later")
	message.SetString(language.English, "Password for username %s has been reset successfully! Please restart ddns-go", "The password of username %s has been reset successfully! Please restart ddns-go")
	message.SetString(language.English, "Need to complete the username and password setting before %s, please restart ddns-go", "Need to complete the username and password setting before %s, please restart ddns-go")
	message.SetString(language.English, "Config file %s does not exist, you can specify the configuration file through -c", "Config file %s does not exist, you can specify the configuration file through -c")

}

func Log(key string, args ...interface{}) {
	log.Println(LogStr(key, args...))
}

func LogStr(key string, args ...interface{}) string {
	return logPrinter.Sprintf(key, args...)
}

func InitLogLang(lang string) string {
	newLang := language.English
	if strings.HasPrefix(lang, "zh") {
		newLang = language.Chinese
	}
	if newLang != logLang {
		logLang = newLang
		logPrinter = message.NewPrinter(logLang)
	}
	return logLang.String()
}
