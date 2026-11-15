package kimi

import (
	"github.com/rainchan/webmux/internal/agentprofile/trustconfig"
)

// Trust describes folder trust without requiring runtime/ACP support.
var Trust = trustconfig.Policy{
	Name:  "kimi",
	Match: func(base string) bool { return base == "kimi" },
	Plan:  planTrust,
}

func planTrust(c trustconfig.Context, dir string) (trustconfig.Plan, error) {
	if _, ok := c.LookupEnv("KIMI_CODE_TRUST_WORKSPACE"); ok {
		return trustconfig.Plan{}, nil
	}
	return trustconfig.Plan{Environment: map[string]string{"KIMI_CODE_TRUST_WORKSPACE": "1"}}, nil
}
