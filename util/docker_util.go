package util

import "os"

// DockerEnvFile Docker file
const DockerEnvFile string = "/.dockerenv"

// IsRunInDocker docker
func IsRunInDocker() bool {
	_, err := os.Stat(DockerEnvFile)
	return err == nil
}
