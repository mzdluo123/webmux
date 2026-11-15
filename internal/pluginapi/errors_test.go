package pluginapi

import (
	"errors"
	"fmt"
	"github.com/rainchan/webmux/internal/push"
	"net/http"
	"testing"
)

func TestPushStatus(t *testing.T) {
	if got := pushStatus(fmt.Errorf("register: %w", push.ErrFull)); got != http.StatusConflict {
		t.Fatalf("full store = %d", got)
	}
	if got := pushStatus(errors.New("disk failed")); got != http.StatusInternalServerError {
		t.Fatalf("save failure = %d", got)
	}
}
