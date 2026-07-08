// Package useragent provides Traccia's default UserAgentParser: a small
// heuristic implementation with zero dependencies. It covers the common
// desktop/mobile browsers and OSes. Swap it for a fuller regex-database
// parser by implementing ports.UserAgentParser and wiring it in cmd/api.
package useragent

import (
	"strings"

	"github.com/antoniojosev/traccia/internal/domain"
)

type HeuristicParser struct{}

func NewHeuristicParser() *HeuristicParser {
	return &HeuristicParser{}
}

func (p *HeuristicParser) Parse(userAgent string) domain.DeviceInfo {
	ua := strings.ToLower(userAgent)
	if ua == "" {
		return domain.DeviceInfo{DeviceType: "unknown", Browser: "unknown", OS: "unknown"}
	}

	return domain.DeviceInfo{
		DeviceType: deviceType(ua),
		Browser:    browser(ua),
		OS:         os(ua),
	}
}

func deviceType(ua string) string {
	switch {
	case containsAny(ua, "bot", "spider", "crawler", "curl", "wget", "postman"):
		return "bot"
	case strings.Contains(ua, "ipad") || strings.Contains(ua, "tablet"):
		return "tablet"
	case strings.Contains(ua, "mobi") || strings.Contains(ua, "android") || strings.Contains(ua, "iphone"):
		return "mobile"
	default:
		return "desktop"
	}
}

func browser(ua string) string {
	switch {
	case strings.Contains(ua, "edg/"):
		return "edge"
	case strings.Contains(ua, "opr/") || strings.Contains(ua, "opera"):
		return "opera"
	case strings.Contains(ua, "firefox"):
		return "firefox"
	case strings.Contains(ua, "chrome") || strings.Contains(ua, "crios"):
		return "chrome"
	case strings.Contains(ua, "safari") && !strings.Contains(ua, "chrome"):
		return "safari"
	default:
		return "other"
	}
}

func os(ua string) string {
	switch {
	case strings.Contains(ua, "windows"):
		return "windows"
	case strings.Contains(ua, "mac os") || strings.Contains(ua, "macos"):
		return "macos"
	case strings.Contains(ua, "android"):
		return "android"
	case strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad") || strings.Contains(ua, "ios"):
		return "ios"
	case strings.Contains(ua, "linux"):
		return "linux"
	default:
		return "other"
	}
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
