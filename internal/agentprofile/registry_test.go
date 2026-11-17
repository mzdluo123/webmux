package agentprofile

import "testing"

func TestEmptyRegistry(t *testing.T) {
	for _, tt := range []struct {
		name     string
		registry *Registry
	}{
		{name: "nil"},
		{name: "empty", registry: NewRegistry()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.registry
			if len(r.Agents()) != 0 || len(r.Titles()) != 0 {
				t.Fatal("empty selection acquired agents")
			}
			if _, ok := r.Lookup("omp"); ok {
				t.Fatal("empty lookup found an agent")
			}
			if _, ok := r.Identify(Signals{}); ok {
				t.Fatal("empty selection identified an agent")
			}
			if got := r.SubmitDelay([]string{"kimi"}); got != 0 {
				t.Fatalf("empty selection supplied delay %v", got)
			}
			for _, argv := range [][]string{nil, {"omp"}, {"sh", "-c", "omp"}} {
				support := r.Check(argv)
				if support.Supported || support.Reason != ReasonUnknownAgent || len(support.Agents) != 0 {
					t.Fatalf("Check(%q) = %+v", argv, support)
				}
				if _, err := r.Parse(argv); err == nil {
					t.Fatalf("Parse(%q) accepted an unregistered command", argv)
				}
			}
			if r.Explain(ReasonUnknownAgent) == "" {
				t.Fatal("missing unsupported explanation")
			}
		})
	}
}
