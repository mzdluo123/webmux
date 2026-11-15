package main

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/rainchan/webmux/internal/hybridresume"
	"github.com/rainchan/webmux/internal/session"
)

type restartResumer struct {
	calls  []string
	cancel context.CancelFunc
}

func (r *restartResumer) Resume(ctx context.Context, id string, in hybridresume.Input) (*session.Session, error) {
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) > restartResumeTimeout {
		return nil, errors.New("missing bounded resume context")
	}
	if in.ClientRequestID != "restart-"+id || in.Side != "" || in.ConfirmWithoutEnv {
		return nil, errors.New("restart changed archived resume policy")
	}
	r.calls = append(r.calls, id)
	if r.cancel != nil {
		r.cancel()
	}
	return nil, errors.New("record stays archived")
}

func TestRestartResumeUsesIndependentPortAndContinuesPastArchivedFailure(t *testing.T) {
	resumer := &restartResumer{}
	resumeRestarted(context.Background(), resumer, []string{"first", "second"})
	if !reflect.DeepEqual(resumer.calls, []string{"first", "second"}) {
		t.Fatalf("resume calls %v", resumer.calls)
	}
}

func TestRestartResumeStopsWhenServerContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resumer := &restartResumer{cancel: cancel}
	resumeRestarted(ctx, resumer, []string{"first", "second"})
	if !reflect.DeepEqual(resumer.calls, []string{"first"}) {
		t.Fatalf("resume calls %v", resumer.calls)
	}
}
