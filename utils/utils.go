package utils

import (
	"strings"
)

func ParseURL(url string) (string, string) {
	arr := strings.Split(url, "://")

	if len(arr) == 1 {
		return "unix", arr[0]
	}

	proto := arr[0]
	if proto == "http" {
		proto = "tcp"
	}

	return proto, arr[1]
}

func SplitPort(portSetting string) (string, string) {
	arr := strings.Split(portSetting, "/")
	return arr[0], arr[1]
}
