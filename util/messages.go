package util

import (
	"github.com/Goon3r/iceMika/conf"
	"strings"
)

func CaptureMessage(message ...string) {
	if conf.Config.SentryDSN == "" {
		return
	}
	msg := strings.Join(message, "")
	if msg == "" {
		return
	}
}
