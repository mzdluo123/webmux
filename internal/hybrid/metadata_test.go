package hybrid

import "testing"

func TestMetadataSnapshotOwnsProviderIdentity(t *testing.T) {
	id := "provider-session"
	service := &Service{metadata: map[string]*Metadata{"runtime": {ConversationID: "conversation", ProviderThreadID: "thread", Side: "acp", State: "active", AgentSessionID: &id, Work: "Read file"}}}
	first := service.Metadata("runtime")
	if first == nil || first.ConversationID != "conversation" || first.ProviderThreadID != "thread" {
		t.Fatalf("metadata %+v", first)
	}
	*first.AgentSessionID = "caller mutation"
	first.Work = "caller mutation"
	second := service.Metadata("runtime")
	if *second.AgentSessionID != "provider-session" || second.Work != "Read file" {
		t.Fatalf("snapshot aliases plugin state: %+v", second)
	}
	service.clearMetadata("runtime")
	if service.Metadata("runtime") != nil || service.Metadata("ordinary") != nil {
		t.Fatal("removed or ordinary runtime has chat metadata")
	}
}
