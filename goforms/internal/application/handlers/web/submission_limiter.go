package web

import (
	"sync"
	"time"

	"golang.org/x/time/rate"

	domainform "github.com/goformx/goforms/internal/domain/form"
)

const (
	maxTrackedSubmissionForms = 4096
	submissionLimiterIdleTTL  = 24 * time.Hour
)

// V1Limits contains admission controls owned by the schema-first HTTP boundary.
type V1Limits struct {
	PublicSubmissionRPS   float64
	PublicSubmissionBurst int
}

func DefaultV1Limits() V1Limits {
	return V1Limits{PublicSubmissionRPS: domainform.DefaultPublicSubmissionRPS,
		PublicSubmissionBurst: domainform.DefaultPublicSubmissionBurst}
}

type formLimiterEntry struct {
	limiter *rate.Limiter
	touched time.Time
}

type submissionLimiter struct {
	mu      sync.Mutex
	entries map[string]*formLimiterEntry
	rps     rate.Limit
	burst   int
	now     func() time.Time
}

func newSubmissionLimiter(limits V1Limits) *submissionLimiter {
	if limits.PublicSubmissionRPS <= 0 {
		limits.PublicSubmissionRPS = domainform.DefaultPublicSubmissionRPS
	}
	if limits.PublicSubmissionBurst <= 0 {
		limits.PublicSubmissionBurst = domainform.DefaultPublicSubmissionBurst
	}
	return &submissionLimiter{
		entries: make(map[string]*formLimiterEntry),
		rps:     rate.Limit(limits.PublicSubmissionRPS), burst: limits.PublicSubmissionBurst, now: time.Now,
	}
}

func (l *submissionLimiter) allow(formID string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	entry := l.entries[formID]
	if entry == nil {
		l.makeRoom(now)
		entry = &formLimiterEntry{limiter: rate.NewLimiter(l.rps, l.burst)}
		l.entries[formID] = entry
	}
	entry.touched = now
	return entry.limiter.AllowN(now, 1)
}

func (l *submissionLimiter) makeRoom(now time.Time) {
	if len(l.entries) < maxTrackedSubmissionForms {
		return
	}
	oldestID := ""
	oldestTime := now
	for formID, entry := range l.entries {
		if now.Sub(entry.touched) >= submissionLimiterIdleTTL {
			delete(l.entries, formID)
			continue
		}
		if oldestID == "" || entry.touched.Before(oldestTime) {
			oldestID, oldestTime = formID, entry.touched
		}
	}
	if len(l.entries) >= maxTrackedSubmissionForms && oldestID != "" {
		delete(l.entries, oldestID)
	}
}
