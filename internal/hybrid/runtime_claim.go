package hybrid

import "crypto/rand"

type writerClaim struct {
	owner string
	refs  int
}

// claimRuntime holds an admission token, without holding a state mutex
// across process work. Nested calls with the same owner share it;
// ordinary resumes each have a distinct owner and cannot race a transition.
func (s *Service) claimRuntime(id, owner string) (func(), error) {
	if owner == "" {
		owner = rand.Text()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrUnavailable
	}
	if s.writerClaims == nil {
		s.writerClaims = map[string]writerClaim{}
	}
	claim := s.writerClaims[id]
	if claim.refs > 0 && claim.owner != owner {
		return nil, ErrSwitching
	}
	claim.owner, claim.refs = owner, claim.refs+1
	s.writerClaims[id] = claim
	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		claim := s.writerClaims[id]
		claim.refs--
		if claim.refs == 0 {
			delete(s.writerClaims, id)
		} else {
			s.writerClaims[id] = claim
		}
	}, nil
}
