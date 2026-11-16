package relay

import (
	"strings"
	"testing"
)

func TestEnrollmentMachineIdentity(t *testing.T) {
	_, public := vectorKey(t)
	tests := []struct {
		name string
		id   MachineID
		want bool
	}{
		{name: "legacy omitted", want: true},
		{name: "canonical", id: MachineID(vectorToken), want: true},
		{name: "too short", id: "abc"},
		{name: "padded", id: MachineID(string(vectorToken) + "=")},
		{name: "noncanonical trailing bits", id: MachineID(strings.Repeat("A", 42) + "B")},
		{name: "invalid alphabet", id: MachineID(strings.Repeat("!", 43))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := EnrollmentRequest{Version: V1, PublicKey: public, MachineID: tt.id}
			if got := request.Validate() == nil; got != tt.want {
				t.Fatalf("enrollment accepted = %v, want %v", got, tt.want)
			}
		})
	}
	if err := (MachineID("")).Validate(); err == nil {
		t.Fatal("empty installation identity accepted outside legacy enrollment")
	}
}

func TestConsoleHostDeleteRequiresGeneration(t *testing.T) {
	request := ConsoleHostDelete{Version: V1, HostID: vectorHost, BindingGeneration: 1}
	if err := request.Validate(); err != nil {
		t.Fatal(err)
	}
	request.BindingGeneration = 0
	requireCode(t, request.Validate(), Invalid)
	request.BindingGeneration = 1
	request.HostID = "not-a-host"
	requireCode(t, request.Validate(), Invalid)
	request.HostID = vectorHost
	request.Version = 2
	requireCode(t, request.Validate(), Incompatible)
}
