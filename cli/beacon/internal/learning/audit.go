package learning

// Append-only security audit log. Security-relevant transitions write an audit
// row inside the SAME transaction as the mutation, so a successful-operation
// record can never outlive a rolled-back mutation and a mutation never commits
// without its audit record. Append-only is enforced through the supported API
// (insert-only); this is NOT tamper-evidence against a privileged local
// administrator, and none is claimed.

import (
	"database/sql"
	"time"
)

// AuditEvent is one security-relevant record. Detail must never contain memory
// content, private keys, tokens, or secrets.
type AuditEvent struct {
	Type      string
	Actor     string
	Project   string
	Target    string
	Outcome   string
	RequestID string
	Detail    string
}

// AuditEntry is a stored audit record.
type AuditEntry struct {
	Seq       int64
	Timestamp string
	Event     string
	Actor     string
	Project   string
	Target    string
	Outcome   string
	RequestID string
	Detail    string
}

func appendAuditTx(tx *sql.Tx, e AuditEvent) error {
	_, err := tx.Exec(`INSERT INTO audit (ts, event, actor, project, target, outcome, request_id, detail)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		time.Now().UTC().Format(time.RFC3339Nano), e.Type, e.Actor, e.Project, e.Target, e.Outcome, e.RequestID, e.Detail)
	return err
}

// AppendAudit records an event outside a mutation transaction (used for denials
// and verification failures, which have no mutation to be consistent with). It
// is best-effort at the caller's discretion; lifecycle events are written inside
// the mutation transaction instead.
func (s *Store) AppendAudit(e AuditEvent) error {
	db, err := s.db()
	if err != nil {
		return err
	}
	return s.busyRetry(func() error {
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if err := appendAuditTx(tx, e); err != nil {
			return err
		}
		return tx.Commit()
	})
}

// auditDenied records a denied lifecycle mutation. It is best-effort: a denial
// has no mutation to be consistent with, so a failed audit write is not fatal.
func (s *Store) auditDenied(candidateID, project string) {
	_ = s.AppendAudit(AuditEvent{Type: "authorization_denied", Project: project,
		Target: candidateID, Outcome: "denied", Detail: "project not authorized"})
}

// AuditEntries returns the audit log in append order (for verification/tests).
func (s *Store) AuditEntries() ([]AuditEntry, error) {
	db, err := s.db()
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT seq, ts, event, COALESCE(actor,''), COALESCE(project,''), COALESCE(target,''),
		COALESCE(outcome,''), COALESCE(request_id,''), COALESCE(detail,'') FROM audit ORDER BY seq`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.Seq, &e.Timestamp, &e.Event, &e.Actor, &e.Project, &e.Target,
			&e.Outcome, &e.RequestID, &e.Detail); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
