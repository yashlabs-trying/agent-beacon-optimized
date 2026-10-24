package learning

import (
	"crypto/ed25519"
	"testing"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

func provMemory(id, project, body string, tags ...string) asymptoteobserve.LearningMemoryV1 {
	return asymptoteobserve.LearningMemoryV1{
		ID: id, CandidateID: "c-" + id, Kind: asymptoteobserve.LearningMemoryKindConvention,
		Title: id, Body: body, Project: asymptoteobserve.LearningProjectV1{ID: project},
		Tags: tags, Evidence: []asymptoteobserve.LearningEvidenceV1{{TraceID: "t-" + id}},
	}
}

func provKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

func TestProvenanceForgedLabelIsNotTrusted(t *testing.T) {
	m := provMemory("m1", "p", "fact", "source:verified")
	reg := NewTrustRegistry()
	if got := Authenticate(m, reg); got.Status != ProvenanceUnverified {
		t.Fatalf("a self-asserted source:verified tag must not authenticate: %+v", got)
	}
}

func TestProvenanceValidAttestationVerified(t *testing.T) {
	pub, priv := provKey(t)
	m := provMemory("m1", "p", "the port is 8080")
	m.Tags = append(m.Tags, SignAttestation(m, "alice", "k1", priv))
	reg := NewTrustRegistry()
	reg.Add("alice", "k1", pub)
	if got := Authenticate(m, reg); got.Status != ProvenanceVerified {
		t.Fatalf("valid attestation not verified: %+v", got)
	}
}

func TestProvenanceTamperedBodyInvalid(t *testing.T) {
	pub, priv := provKey(t)
	m := provMemory("m1", "p", "the port is 8080")
	m.Tags = append(m.Tags, SignAttestation(m, "alice", "k1", priv))
	m.Body = "the port is 9999" // tamper after signing
	reg := NewTrustRegistry()
	reg.Add("alice", "k1", pub)
	if got := Authenticate(m, reg); got.Status != ProvenanceInvalid {
		t.Fatalf("tampered content must be invalid: %+v", got)
	}
}

func TestProvenanceInvalidSignatureRejected(t *testing.T) {
	_, priv := provKey(t)
	otherPub, _ := provKey(t)
	m := provMemory("m1", "p", "the port is 8080")
	m.Tags = append(m.Tags, SignAttestation(m, "alice", "k1", priv))
	reg := NewTrustRegistry()
	reg.Add("alice", "k1", otherPub) // registered key does not match the signer
	if got := Authenticate(m, reg); got.Status != ProvenanceInvalid {
		t.Fatalf("signature from a non-matching key must be invalid: %+v", got)
	}
}

func TestProvenanceRevokedKeyNotTrusted(t *testing.T) {
	pub, priv := provKey(t)
	m := provMemory("m1", "p", "fact")
	m.Tags = append(m.Tags, SignAttestation(m, "alice", "k1", priv))
	reg := NewTrustRegistry()
	reg.Add("alice", "k1", pub)
	reg.Remove("alice", "k1") // revoke
	if got := Authenticate(m, reg); got.Status == ProvenanceVerified {
		t.Fatalf("revoked key still granted trust: %+v", got)
	}
}

func TestProvenanceCrossProjectRejected(t *testing.T) {
	pub, priv := provKey(t)
	m := provMemory("m1", "projectA", "fact")
	m.Tags = append(m.Tags, SignAttestation(m, "alice", "k1", priv))
	// Same content, but presented under a different project scope.
	reused := m
	reused.Project.ID = "projectB"
	reg := NewTrustRegistry()
	reg.Add("alice", "k1", pub)
	if got := Authenticate(reused, reg); got.Status != ProvenanceInvalid {
		t.Fatalf("cross-project attestation must be invalid: %+v", got)
	}
}

func TestProvenanceEmptyRegistryFailsSafe(t *testing.T) {
	_, priv := provKey(t)
	m := provMemory("m1", "p", "fact")
	m.Tags = append(m.Tags, SignAttestation(m, "alice", "k1", priv))
	if got := Authenticate(m, NewTrustRegistry()); got.Status == ProvenanceVerified {
		t.Fatalf("empty registry must not verify anything: %+v", got)
	}
}

func TestSelectTrustContextPopulatesAuthenticatedIDs(t *testing.T) {
	pub, priv := provKey(t)
	good := provMemory("good", "p", "the port is 8080")
	good.Tags = append(good.Tags, SignAttestation(good, "alice", "k1", priv))
	forged := provMemory("forged", "p", "the port is 9999", "source:verified")
	reg := NewTrustRegistry()
	reg.Add("alice", "k1", pub)
	res := SelectTrustContext([]asymptoteobserve.LearningMemoryV1{good, forged}, "port",
		TrustContextOptions{EnableTrust: true, Trust: reg})
	if len(res.Signals.AuthenticatedIDs) != 1 || res.Signals.AuthenticatedIDs[0] != "good" {
		t.Fatalf("only the signed memory should be authenticated: %+v", res.Signals.AuthenticatedIDs)
	}
	for _, id := range res.Signals.AuthenticatedIDs {
		if id == "forged" {
			t.Fatalf("a self-asserted label was authenticated")
		}
	}
}
