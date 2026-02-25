package util

import (
	"os"
)

const ConfigFilePathENV = "DDNS_CONFIG_FILE_PATH"

// GetConfigFilePath getconfig file path
func GetConfigFilePath() string {
	configFilePath := os.Getenv(ConfigFilePathENV)
	if configFilePath != "" {
		return configFilePath
	}
	return GetConfigFilePathDefault()
}

// GetConfigFilePathDefault getdefault config file path
func GetConfigFilePathDefault() string {
	dir, err := os.UserHomeDir()
	if err != nil {
		// log.Println("Getting Home directory failed: ", err)
		return "../.ddns_go_config.yaml"
	}
	return dir + string(os.PathSeparator) + ".ddns_go_config.yaml"
}
