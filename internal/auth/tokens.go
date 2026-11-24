package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

func (s *Service) touchSessionLocked(index int, now time.Time) {
	id := s.doc.Sessions[index].ID
	if prev, ok := s.lastTouch[id]; ok && now.Sub(prev) < sessionTouchPeriod {
		return
	}
	s.lastTouch[id] = now
	next := s.cloneDocLocked()
	next.Sessions[index].LastUsedAt = now.Format(time.RFC3339Nano)
	next.Sessions[index].ExpiresAt = now.Add(sessionTTL).Format(time.RFC3339Nano)
	_ = s.commitLocked(next)
}

func (s *Service) touchAPITokenLocked(index int, now time.Time) {
	id := "pat:" + s.doc.APITokens[index].ID
	if prev, ok := s.lastTouch[id]; ok && now.Sub(prev) < sessionTouchPeriod {
		return
	}
	s.lastTouch[id] = now
	next := s.cloneDocLocked()
	next.APITokens[index].LastUsedAt = now.Format(time.RFC3339Nano)
	_ = s.commitLocked(next)
}

func (s *Service) ensureSetupLocked() {
	now := s.now().UTC()
	if s.setup != nil && now.Before(s.setup.expiresAt) {
		return
	}
	display := formatDisplayCode(12)
	s.setup = &setupEntry{
		code:      randomToken(18),
		display:   display,
		expiresAt: now.Add(setupCodeTTL),
	}
}

func (s *Service) prunePendingLocked(now time.Time) {
	for k, v := range s.tickets {
		if now.After(v.expiresAt) {
			delete(s.tickets, k)
		}
	}
	for k, v := range s.challenges {
		if now.After(v.expiresAt) {
			delete(s.challenges, k)
		}
	}
	for k, v := range s.pairings {
		if now.After(v.expiresAt) {
			delete(s.pairings, k)
		}
	}
	for k, v := range s.stepUps {
		if now.After(v.expiresAt) {
			delete(s.stepUps, k)
		}
	}
	trimMap(s.tickets, maxPendingItems)
	trimMap(s.challenges, maxPendingItems)
	trimMap(s.pairings, maxPendingItems)
	trimMap(s.stepUps, maxPendingItems)
}

func trimMap[V any](m map[string]V, limit int) {
	for k := range m {
		if len(m) < limit {
			return
		}
		delete(m, k)
	}
}

func (s *Service) pruneExpiredSessionsLocked(now time.Time) {
	kept := s.doc.Sessions[:0]
	for _, sess := range s.doc.Sessions {
		if exp, err := time.Parse(time.RFC3339Nano, sess.ExpiresAt); err == nil && exp.After(now) {
			kept = append(kept, sess)
		}
	}
	s.doc.Sessions = kept
}

func (s *Service) remainingRecoveryLocked() int {
	n := 0
	for _, r := range s.doc.RecoveryCodes {
		if r.UsedAt == "" {
			n++
		}
	}
	return n
}

func (s *Service) cloneDocLocked() storedAuth {
	out := s.doc
	out.Passkeys = append([]PasskeyRecord{}, s.doc.Passkeys...)
	out.Sessions = append([]SessionRecord{}, s.doc.Sessions...)
	out.RecoveryCodes = append([]RecoveryRecord{}, s.doc.RecoveryCodes...)
	out.APITokens = append([]APITokenRecord{}, s.doc.APITokens...)
	return out
}

func (s *Service) commitLocked(next storedAuth) error {
	if s.handle == nil {
		return ErrStorageUnavailable
	}
	if err := s.handle.Commit(next); err != nil {
		return err
	}
	s.doc = normalizeDoc(next)
	return nil
}

func randomToken(nBytes int) string {
	b := make([]byte, nBytes)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func hashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func constantTimeStringEqual(a, b string) bool {
	ha := sha256.Sum256([]byte(a))
	hb := sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(ha[:], hb[:]) == 1
}

// unambiguous Crockford-like alphabet (no 0/O/1/I/L) for hand-typed codes.
const codeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

func formatDisplayCode(chars int) string {
	b := make([]byte, chars)
	_, _ = rand.Read(b)
	var out strings.Builder
	for i := range chars {
		if i > 0 && i%4 == 0 {
			out.WriteByte('-')
		}
		out.WriteByte(codeAlphabet[int(b[i])%len(codeAlphabet)])
	}
	return out.String()
}

func normalizeCodeInput(raw string) string {
	s := strings.ToUpper(strings.TrimSpace(raw))
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, " ", "")
	return s
}

func sanitizeName(raw, fallback string, maxLen int) string {
	s := strings.TrimSpace(raw)
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
	if s == "" {
		s = fallback
	}
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	return s
}

func (s *Service) appendSessionLocked(doc *storedAuth, name, rpID string, now time.Time) (string, SessionRecord) {
	active := doc.Sessions[:0]
	for _, existing := range doc.Sessions {
		if exp, err := time.Parse(time.RFC3339Nano, existing.ExpiresAt); err == nil && exp.After(now) {
			active = append(active, existing)
		}
	}
	doc.Sessions = active
	if len(doc.Sessions) >= maxSessions {
		oldestIdx := 0
		for i := 1; i < len(doc.Sessions); i++ {
			if doc.Sessions[i].LastUsedAt < doc.Sessions[oldestIdx].LastUsedAt {
				oldestIdx = i
			}
		}
		doc.Sessions = append(doc.Sessions[:oldestIdx], doc.Sessions[oldestIdx+1:]...)
	}
	secret := randomToken(32)
	rec := SessionRecord{
		ID:         randomToken(12),
		TokenHash:  hashSecret(secret),
		Name:       sanitizeName(name, "Browser", 120),
		RPID:       rpID,
		CreatedAt:  now.Format(time.RFC3339Nano),
		LastUsedAt: now.Format(time.RFC3339Nano),
		ExpiresAt:  now.Add(sessionTTL).Format(time.RFC3339Nano),
	}
	doc.Sessions = append(doc.Sessions, rec)
	return secret, rec
}

func generateRecoveryBatch() ([]string, []RecoveryRecord) {
	plain := make([]string, recoveryCodeCount)
	records := make([]RecoveryRecord, recoveryCodeCount)
	for i := range recoveryCodeCount {
		code := formatDisplayCode(8)
		plain[i] = code
		records[i] = RecoveryRecord{
			ID:   fmt.Sprintf("rc_%d_%s", i+1, randomToken(6)),
			Hash: hashSecret(normalizeCodeInput(code)),
		}
	}
	return plain, records
}
