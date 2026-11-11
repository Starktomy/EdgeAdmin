package csrf

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"sync"
	"time"
)

const tokenLifetime = 30 * time.Minute
const maxTokens = 100000
const maxSessionTokens = 64

var sharedTokenManager = NewTokenManager()

type tokenRecord struct {
	sessionID string
	expiresAt time.Time
}

type TokenManager struct {
	tokenMap      map[string]tokenRecord
	sessionTokens map[string][]string
	locker        sync.Mutex
	now           func() time.Time
	nextClean     time.Time
}

func NewTokenManager() *TokenManager {
	return &TokenManager{tokenMap: map[string]tokenRecord{}, sessionTokens: map[string][]string{}, now: time.Now}
}

func (this *TokenManager) Generate(sessionID string) (string, error) {
	if len(sessionID) == 0 {
		return "", errors.New("a browser session is required")
	}
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(random[:])
	this.locker.Lock()
	defer this.locker.Unlock()
	now := this.now()
	if !now.Before(this.nextClean) || len(this.tokenMap) >= maxTokens {
		this.clean(now)
		this.nextClean = now.Add(time.Minute)
	}
	if len(this.tokenMap) >= maxTokens {
		return "", errors.New("too many pending form tokens")
	}
	pending := this.sessionTokens[sessionID]
	active := pending[:0]
	for _, old := range pending {
		if _, ok := this.tokenMap[old]; ok {
			active = append(active, old)
		}
	}
	if len(active) >= maxSessionTokens {
		delete(this.tokenMap, active[0])
		active = active[1:]
	}
	this.sessionTokens[sessionID] = append(active, token)
	this.tokenMap[token] = tokenRecord{sessionID: sessionID, expiresAt: now.Add(tokenLifetime)}
	return token, nil
}

func (this *TokenManager) Consume(token string, sessionID string) bool {
	if len(token) != 43 || len(sessionID) == 0 {
		return false
	}
	this.locker.Lock()
	defer this.locker.Unlock()
	record, ok := this.tokenMap[token]
	if !ok || record.sessionID != sessionID {
		return false
	}
	delete(this.tokenMap, token)
	return this.now().Before(record.expiresAt)
}

func (this *TokenManager) Clean() {
	this.locker.Lock()
	defer this.locker.Unlock()
	this.clean(this.now())
}

func (this *TokenManager) clean(now time.Time) {
	for token, record := range this.tokenMap {
		if !now.Before(record.expiresAt) {
			delete(this.tokenMap, token)
		}
	}
	for sessionID, pending := range this.sessionTokens {
		active := pending[:0]
		for _, token := range pending {
			if _, ok := this.tokenMap[token]; ok {
				active = append(active, token)
			}
		}
		if len(active) == 0 {
			delete(this.sessionTokens, sessionID)
		} else {
			this.sessionTokens[sessionID] = active
		}
	}
}
