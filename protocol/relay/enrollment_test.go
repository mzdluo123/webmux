package relay

import (
	"encoding/json"
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
		{name: "missing machine identity"},
		{name: "canonical", id: MachineID(vectorToken), want: true},
		{name: "too short", id: "abc"},
		{name: "padded", id: MachineID(string(vectorToken) + "=")},
		{name: "noncanonical trailing bits", id: MachineID(strings.Repeat("A", 42) + "B")},
		{name: "invalid alphabet", id: MachineID(strings.Repeat("!", 43))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := EnrollmentRequest{Version: BindingAPIVersion, PublicKey: public, MachineID: tt.id}
			if got := request.Validate() == nil; got != tt.want {
				t.Fatalf("enrollment accepted = %v, want %v", got, tt.want)
			}
		})
	}
	if err := (MachineID("")).Validate(); err == nil {
		t.Fatal("empty installation identity accepted")
	}
	requireCode(t, (EnrollmentRequest{Version: V1, PublicKey: public, MachineID: MachineID(vectorToken)}).Validate(), Incompatible)
	requireCode(t, (EnrollmentPoll{Version: V1, Handle: vectorToken}).Validate(), Incompatible)
}

func TestConsoleHostDeleteRequiresGeneration(t *testing.T) {
	request := ConsoleHostDelete{Version: BindingAPIVersion, HostID: vectorHost, BindingGeneration: 1}
	if err := request.Validate(); err != nil {
		t.Fatal(err)
	}
	request.BindingGeneration = 0
	requireCode(t, request.Validate(), Invalid)
	request.BindingGeneration = 1
	request.HostID = "not-a-host"
	requireCode(t, request.Validate(), Invalid)
	request.HostID = vectorHost
	request.Version = V1
	requireCode(t, request.Validate(), Incompatible)
}

func TestEnrollmentRejectsRetiredRebindField(t *testing.T) {
	_, public := vectorKey(t)
	legacy := struct {
		Version   Version   `json:"version"`
		PublicKey PublicKey `json:"publicKey"`
		MachineID MachineID `json:"machineId"`
		Rebind    bool      `json:"rebind"`
	}{Version: BindingAPIVersion, PublicKey: public, MachineID: MachineID(vectorToken)}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	var request EnrollmentRequest
	requireCode(t, Decode(data, &request, MaxAuthBytes), Invalid)
}

func TestBindingDeletionRequiresCommittedRemoval(t *testing.T) {
	result := BindingDeletionResult{Version: BindingAPIVersion, RevocationID: vectorToken, Deleted: true}
	if err := result.Validate(); err != nil {
		t.Fatal(err)
	}
	result.Deleted = false
	requireCode(t, result.Validate(), Invalid)
	result.Deleted = true
	result.RevocationID = ""
	requireCode(t, result.Validate(), Invalid)
	result.RevocationID = vectorToken
	result.Version = V1
	requireCode(t, result.Validate(), Incompatible)
}
