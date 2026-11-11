package loginutils

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/xlzd/gotp"
)

const challengeLifetime = 5 * time.Minute
const challengeAttempts = 5
const accountAttemptWindow = 15 * time.Minute
const accountAttempts = 10
const maxChallenges = 20000

var OTPChallenges = NewChallengeManager()

type Challenge struct {
	AdminID   int64
	Remember  bool
	browserID string
	expiresAt time.Time
	attempts  int
}

type attemptRecord struct {
	attempts  int
	expiresAt time.Time
}

type ChallengeManager struct {
	mu         sync.Mutex
	challenges map[string]Challenge
	failures   map[int64]attemptRecord
	usedSteps  map[int64]int64
	now        func() time.Time
	nextClean  time.Time
}

func NewChallengeManager() *ChallengeManager {
	return &ChallengeManager{challenges: map[string]Challenge{}, failures: map[int64]attemptRecord{}, usedSteps: map[int64]int64{}, now: time.Now}
}

func (m *ChallengeManager) Create(browserID string, adminID int64, remember bool) (string, error) {
	if len(browserID) == 0 || adminID <= 0 {
		return "", errors.New("invalid login challenge")
	}
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	id := base64.RawURLEncoding.EncodeToString(random[:])
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	m.clean(now)
	if failure := m.failures[adminID]; failure.attempts >= accountAttempts && now.Before(failure.expiresAt) {
		return "", errors.New("too many verification attempts; try again later")
	}
	for oldID, challenge := range m.challenges {
		if challenge.browserID == browserID {
			delete(m.challenges, oldID)
		}
	}
	if len(m.challenges) >= maxChallenges {
		return "", errors.New("too many pending login challenges")
	}
	m.challenges[id] = Challenge{AdminID: adminID, Remember: remember, browserID: browserID, expiresAt: now.Add(challengeLifetime)}
	return id, nil
}

func (m *ChallengeManager) Details(id, browserID string) (Challenge, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.details(id, browserID, m.now())
}

func (m *ChallengeManager) details(id, browserID string, now time.Time) (Challenge, bool) {
	challenge, ok := m.challenges[id]
	if !ok || browserID == "" || challenge.browserID != browserID {
		return Challenge{}, false
	}
	if !now.Before(challenge.expiresAt) || challenge.attempts >= challengeAttempts {
		delete(m.challenges, id)
		return Challenge{}, false
	}
	failure := m.failures[challenge.AdminID]
	if failure.attempts >= accountAttempts && now.Before(failure.expiresAt) {
		return Challenge{}, false
	}
	return challenge, true
}

// Verify checks and consumes a browser-bound challenge under the same lock.
// A TOTP time step is accepted once per administrator in this process.
func (m *ChallengeManager) Verify(id, browserID, secret, code string) (Challenge, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	challenge, ok := m.details(id, browserID, now)
	if !ok {
		return Challenge{}, false
	}
	challenge.attempts++
	step := now.Unix() / 30
	decodedSecret, secretErr := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.TrimRight(secret, "="))
	valid := secretErr == nil && len(decodedSecret) >= 10 && len(secret) <= 128 && len(code) == 6 && subtle.ConstantTimeCompare([]byte(gotp.NewDefaultTOTP(secret).At(now.Unix())), []byte(code)) == 1
	if valid && m.usedSteps[challenge.AdminID] < step {
		delete(m.challenges, id)
		delete(m.failures, challenge.AdminID)
		m.usedSteps[challenge.AdminID] = step
		return challenge, true
	}
	failure := m.failures[challenge.AdminID]
	if !now.Before(failure.expiresAt) {
		failure = attemptRecord{expiresAt: now.Add(accountAttemptWindow)}
	}
	failure.attempts++
	m.failures[challenge.AdminID] = failure
	if challenge.attempts >= challengeAttempts {
		delete(m.challenges, id)
	} else {
		m.challenges[id] = challenge
	}
	return Challenge{}, false
}

func (m *ChallengeManager) clean(now time.Time) {
	if now.Before(m.nextClean) {
		return
	}
	for id, challenge := range m.challenges {
		if !now.Before(challenge.expiresAt) {
			delete(m.challenges, id)
		}
	}
	for id, failure := range m.failures {
		if !now.Before(failure.expiresAt) {
			delete(m.failures, id)
		}
	}
	for id, step := range m.usedSteps {
		if step < now.Unix()/30-2 {
			delete(m.usedSteps, id)
		}
	}
	m.nextClean = now.Add(time.Minute)
}
