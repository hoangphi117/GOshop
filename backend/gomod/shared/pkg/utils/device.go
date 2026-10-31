package utils

import "strings"

// ParseDeviceName provides a lightweight, dependency-free device and browser summarizer from User-Agent.
func ParseDeviceName(ua string) string {
	if ua == "" {
		return "Unknown Device"
	}

	if strings.Contains(ua, "Postman") {
		return "Postman API Client"
	}

	browser := "Web Browser"
	if strings.Contains(ua, "Edg/") || strings.Contains(ua, "Edge/") {
		browser = "Edge"
	} else if strings.Contains(ua, "Chrome/") && !strings.Contains(ua, "Chromium/") {
		browser = "Chrome"
	} else if strings.Contains(ua, "Safari/") && !strings.Contains(ua, "Chrome/") {
		browser = "Safari"
	} else if strings.Contains(ua, "Firefox/") {
		browser = "Firefox"
	} else if strings.Contains(ua, "Opera/") || strings.Contains(ua, "OPR/") {
		browser = "Opera"
	}

	os := "Unknown OS"
	if strings.Contains(ua, "Windows NT 10.0") || strings.Contains(ua, "Windows") {
		os = "Windows"
	} else if strings.Contains(ua, "iPhone") {
		os = "iPhone (iOS)"
	} else if strings.Contains(ua, "iPad") {
		os = "iPad (iPadOS)"
	} else if strings.Contains(ua, "Macintosh") || strings.Contains(ua, "Mac OS X") {
		os = "macOS"
	} else if strings.Contains(ua, "Android") {
		os = "Android"
	} else if strings.Contains(ua, "Linux") {
		os = "Linux"
	}

	return browser + " on " + os
}
