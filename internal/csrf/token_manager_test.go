package csrf

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTokensAreRandomAndBoundToSession(t *testing.T) {
	manager := NewTokenManager()
	a, err := manager.Generate("browser-a")
	if err != nil {
		t.Fatal(err)
	}
	b, err := manager.Generate("browser-a")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("form tokens must be independently random")
	}
	if manager.Consume(a, "browser-b") {
		t.Fatal("token accepted in another browser session")
	}
	if !manager.Consume(a, "browser-a") {
		t.Fatal("another browser must not consume the owner's token")
	}
	if manager.Consume(a, "browser-a") {
		t.Fatal("token accepted twice")
	}
}

func TestExpiredTokenIsRejectedWithoutCleanup(t *testing.T) {
	manager := NewTokenManager()
	now := time.Now()
	manager.now = func() time.Time { return now }
	token, err := manager.Generate("browser")
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(tokenLifetime)
	if manager.Consume(token, "browser") {
		t.Fatal("expired token accepted")
	}
}

func TestConcurrentTokenConsumptionSucceedsOnce(t *testing.T) {
	manager := NewTokenManager()
	token, err := manager.Generate("browser")
	if err != nil {
		t.Fatal(err)
	}
	var successes atomic.Int32
	var group sync.WaitGroup
	for i := 0; i < 32; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			if manager.Consume(token, "browser") {
				successes.Add(1)
			}
		}()
	}
	group.Wait()
	if successes.Load() != 1 {
		t.Fatalf("accepted %d times", successes.Load())
	}
}

func TestUnusedReadRequestTokensCannotFillGlobalPool(t *testing.T) {
	manager := NewTokenManager()
	for i := 0; i < maxSessionTokens*3; i++ {
		if _, err := manager.Generate("browser"); err != nil {
			t.Fatal(err)
		}
	}
	if len(manager.tokenMap) != maxSessionTokens {
		t.Fatalf("unbounded tokens for one browser: %d", len(manager.tokenMap))
	}
}
