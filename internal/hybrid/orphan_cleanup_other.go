//go:build !linux && !darwin

package hybrid

import "github.com/rainchan/webmux/internal/runtime"

func cleanupRecordOrphans(_ string, _ int, _ *runtime.Identity) bool {
	return true
}
