package learning

// Phase 3 (V7 hardening): authenticated memory provenance.
//
// Threat model (declared): a single-user local deployment where any process that
// can write the store can also write arbitrary memory text and tags. A tag such as
// `source:verified` is therefore NOT proof of anything: it is attacker-writable.
// Elevated trust must come from a cryptographic attestation bound to the exact
// approved content and to an approver identity whose key is held in an
// operator-controlled trust registry.
//
// This file provides the schema, verification boundary, and fail-safe behaviour:
//
//   - ContentDigest binds an attestation to the exact memory content and scope.
//   - An attestation is an Ed25519 signature over that digest by a registered key.
//   - Authenticate grants Verified ONLY when the signature validates against a
//     trusted key and the bound project matches. Every other case (no attestation,
//     unknown/revoked key, bad signature, tampered content, wrong project) is
//     Unverified or Invalid — never Verified.
//   - A self-asserted `source:verified` tag alone is never enough.
//
// The full authentication gate is BLOCKED in this environment: there is no
// protected key authority or MCP client identity to authenticate, so the default
// registry is empty and every record is Unverified. The mechanism, boundaries,
// and adversarial tests are implemented so it can be enabled once a protected
// signing authority exists.

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

// AttestationTagPrefix marks a reserved tag that carries a provenance attestation.
// It is deliberately distinct from the informational `source:` tags so the two can
// never be confused.
const AttestationTagPrefix = "attest:v1:"

// ProvenanceStatus is the outcome of authenticating a memory's provenance.
type ProvenanceStatus string

const (
	// ProvenanceVerified means a valid signature by a trusted key over the exact
	// content and scope.
	ProvenanceVerified ProvenanceStatus = "verified"
	// ProvenanceUnverified means no usable attestation (missing, unknown key, or no
	// registry). It is the fail-safe default and never grants elevated trust.
	ProvenanceUnverified ProvenanceStatus = "unverified"
	// ProvenanceInvalid means an attestation was present but failed verification
	// (bad signature, tampered content, or scope mismatch).
	ProvenanceInvalid ProvenanceStatus = "invalid"
)

// Provenance is the authentication result for one memory.
type Provenance struct {
	Status   ProvenanceStatus `json:"status"`
	Approver string           `json:"approver,omitempty"`
	KeyID    string           `json:"key_id,omitempty"`
	Reason   string           `json:"reason,omitempty"`
}

// attestationPayload is the signed, content-bound attestation.
type attestationPayload struct {
	V        int    `json:"v"`
	Approver string `json:"approver"`
	KeyID    string `json:"key_id"`
	Project  string `json:"project"`
	Digest   string `json:"digest"` // hex SHA-256 of the memory content
	Sig      string `json:"sig"`    // base64 Ed25519 signature over the digest bytes
}

// ContentDigest returns a stable SHA-256 over the authoritative, scope-defining
// fields of a memory. Any change to id, project, kind, title, or body changes the
// digest, so a valid attestation cannot be reused for modified content.
func ContentDigest(m asymptoteobserve.LearningMemoryV1) []byte {
	h := sha256.New()
	for _, part := range []string{m.ID, m.Project.ID, m.Kind, m.Title, m.Body} {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return h.Sum(nil)
}

// TrustRegistry maps (approver, key_id) to a trusted Ed25519 public key. It is the
// authority that decides which signers are trusted; an empty registry trusts
// nothing, which is the safe default.
type TrustRegistry struct {
	keys map[string]ed25519.PublicKey
}

// NewTrustRegistry builds a registry from approver/key-id/public-key triples.
func NewTrustRegistry() *TrustRegistry { return &TrustRegistry{keys: map[string]ed25519.PublicKey{}} }

// Add registers a trusted key. A later Add for the same (approver,keyID) replaces
// the previous key (rotation); removing a key revokes it.
func (r *TrustRegistry) Add(approver, keyID string, pub ed25519.PublicKey) {
	r.keys[approver+"\x00"+keyID] = pub
}

// Remove revokes a key.
func (r *TrustRegistry) Remove(approver, keyID string) {
	delete(r.keys, approver+"\x00"+keyID)
}

// IsEmpty reports whether the registry trusts no signers.
func (r *TrustRegistry) IsEmpty() bool { return r == nil || len(r.keys) == 0 }

func (r *TrustRegistry) lookup(approver, keyID string) (ed25519.PublicKey, bool) {
	if r == nil {
		return nil, false
	}
	k, ok := r.keys[approver+"\x00"+keyID]
	return k, ok
}

// SignAttestation produces the reserved tag that attests a memory's content and
// scope under an approver identity. It is used by an operator/approval service
// that holds the private key; tests use it too.
func SignAttestation(m asymptoteobserve.LearningMemoryV1, approver, keyID string, priv ed25519.PrivateKey) string {
	digest := ContentDigest(m)
	payload := attestationPayload{
		V: 1, Approver: approver, KeyID: keyID, Project: m.Project.ID,
		Digest: hex.EncodeToString(digest),
		Sig:    base64.StdEncoding.EncodeToString(ed25519.Sign(priv, digest)),
	}
	blob, _ := json.Marshal(payload)
	return AttestationTagPrefix + base64.StdEncoding.EncodeToString(blob)
}

// AttestationTag returns the attestation tag carried by a memory, if any.
func AttestationTag(m asymptoteobserve.LearningMemoryV1) (string, bool) {
	for _, tag := range m.Tags {
		if strings.HasPrefix(tag, AttestationTagPrefix) {
			return tag, true
		}
	}
	return "", false
}

// Authenticate verifies a memory's provenance against the registry. It grants
// Verified only for a valid signature by a trusted key over the exact content and
// scope. Everything else is Unverified (no attestation / unknown key) or Invalid
// (present but failing). It never treats a plain `source:` tag as proof.
func Authenticate(m asymptoteobserve.LearningMemoryV1, reg *TrustRegistry) Provenance {
	tag, ok := AttestationTag(m)
	if !ok {
		return Provenance{Status: ProvenanceUnverified, Reason: "no attestation; labels are not proof"}
	}
	blob, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(tag, AttestationTagPrefix))
	if err != nil {
		return Provenance{Status: ProvenanceInvalid, Reason: "malformed attestation encoding"}
	}
	var payload attestationPayload
	if err := json.Unmarshal(blob, &payload); err != nil || payload.V != 1 {
		return Provenance{Status: ProvenanceInvalid, Reason: "malformed attestation payload"}
	}
	if payload.Project != m.Project.ID {
		return Provenance{Status: ProvenanceInvalid, Approver: payload.Approver, KeyID: payload.KeyID,
			Reason: "attestation scope does not match memory project"}
	}
	digest := ContentDigest(m)
	if payload.Digest != hex.EncodeToString(digest) {
		return Provenance{Status: ProvenanceInvalid, Approver: payload.Approver, KeyID: payload.KeyID,
			Reason: "content digest does not match (memory was modified)"}
	}
	pub, trusted := reg.lookup(payload.Approver, payload.KeyID)
	if !trusted {
		return Provenance{Status: ProvenanceUnverified, Approver: payload.Approver, KeyID: payload.KeyID,
			Reason: "signer is not a trusted/active key"}
	}
	sig, err := base64.StdEncoding.DecodeString(payload.Sig)
	if err != nil || !ed25519.Verify(pub, digest, sig) {
		return Provenance{Status: ProvenanceInvalid, Approver: payload.Approver, KeyID: payload.KeyID,
			Reason: "signature verification failed"}
	}
	return Provenance{Status: ProvenanceVerified, Approver: payload.Approver, KeyID: payload.KeyID}
}

// Describe is a compact human string for diagnostics.
func (p Provenance) Describe() string {
	if p.Reason == "" {
		return string(p.Status)
	}
	return fmt.Sprintf("%s (%s)", p.Status, p.Reason)
}
