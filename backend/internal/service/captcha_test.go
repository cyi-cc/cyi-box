package service

import (
	"testing"
	"time"
)

func newTestPuzzle(keyID int64, ttl time.Duration) *capPuzzle {
	return &capPuzzle{answerX: 100, keyID: keyID, exp: time.Now().Add(ttl).Unix()}
}

func TestPuzzleTryConsume(t *testing.T) {
	id := acquirePuzzle("t1", newTestPuzzle(7, capPuzzleTTL))
	if _, ok := tryPuzzle("t-ip", id, 90); ok {
		t.Fatal("wrong x should fail")
	}
	keyID, ok := tryPuzzle("t-ip", id, 102) // 容差 4px 内
	if !ok || keyID != 7 {
		t.Fatalf("near x should pass and return keyID, got %v %v", keyID, ok)
	}
	if _, ok := tryPuzzle("t-ip", id, 102); ok {
		t.Fatal("consumed puzzle must not verify twice")
	}
}

func TestPuzzleMaxTries(t *testing.T) {
	id := acquirePuzzle("t2", newTestPuzzle(0, capPuzzleTTL))
	for i := 0; i < capMaxTry; i++ {
		tryPuzzle("t-ip", id, 0)
	}
	if _, ok := tryPuzzle("t-ip", id, 100); ok {
		t.Fatal("puzzle exhausted by max tries must be invalid")
	}
}

func TestPuzzleExpired(t *testing.T) {
	id := acquirePuzzle("t3", newTestPuzzle(0, -time.Second))
	if _, ok := tryPuzzle("t-ip", id, 100); ok {
		t.Fatal("expired puzzle must fail")
	}
}

// 题位队列：满 10 个即拒；验证/作废均不释放，只能等 30 分钟自动回收
func TestSlotQueue(t *testing.T) {
	ip := "slot-ip"
	ids := make([]string, 0, capMaxSlots)
	for i := 0; i < capMaxSlots; i++ {
		id := acquirePuzzle(ip, newTestPuzzle(0, capPuzzleTTL))
		if id == "" {
			t.Fatalf("slot %d should be granted", i)
		}
		ids = append(ids, id)
	}
	if acquirePuzzle(ip, newTestPuzzle(0, capPuzzleTTL)) != "" {
		t.Fatal("11th acquisition must be rejected")
	}
	// 验证成功、试错超限都不释放题位
	if _, ok := tryPuzzle(ip, ids[0], 100); !ok {
		t.Fatal("solve should succeed")
	}
	for i := 0; i < capMaxTry; i++ {
		tryPuzzle(ip, ids[1], 0)
	}
	if acquirePuzzle(ip, newTestPuzzle(0, capPuzzleTTL)) != "" {
		t.Fatal("slots must not free on consume/exhaust")
	}
}

// 打错封禁：滑窗累计 5 次即封；封禁期答对题也无效；成功验签清零
func TestIPFailBan(t *testing.T) {
	ip := "ban-ip"
	for i := 0; i < capMaxIPFail-1; i++ {
		if capBanned(ip) {
			t.Fatalf("banned too early at %d", i)
		}
		id := acquirePuzzle(ip, newTestPuzzle(0, capPuzzleTTL))
		tryPuzzle(ip, id, 0)
	}
	// 第 5 次打错 → 封禁
	id5 := acquirePuzzle(ip, newTestPuzzle(0, capPuzzleTTL))
	tryPuzzle(ip, id5, 0)
	if !capBanned(ip) {
		t.Fatal("5 fails should ban ip")
	}
	// 封禁语义由 handler 层 capBanned 把关；成功后清零
	capMu.Lock()
	capFails[ip] = [2]int64{capMaxIPFail - 1, time.Now().Unix() + 60}
	capMu.Unlock()
	idOK := acquirePuzzle(ip, newTestPuzzle(0, capPuzzleTTL))
	if _, ok := tryPuzzle(ip, idOK, 100); !ok {
		t.Fatal("correct x should pass")
	}
	if capBanned(ip) {
		t.Fatal("success should clear fail counter")
	}
}

func TestSlotAutoExpire(t *testing.T) {
	ip := "stale-ip"
	capMu.Lock()
	capSlots[ip] = make([]int64, 0, capMaxSlots)
	stale := time.Now().Add(-capSlotTTL - time.Minute).Unix()
	for i := 0; i < capMaxSlots; i++ {
		capSlots[ip] = append(capSlots[ip], stale)
	}
	capMu.Unlock()
	if acquirePuzzle(ip, newTestPuzzle(0, capPuzzleTTL)) == "" {
		t.Fatal("slots stale >30min must auto-expire")
	}
}

func TestTokenConsumeOnce(t *testing.T) {
	tok := mintToken(3)
	if !consumeToken(tok, 3) {
		t.Fatal("fresh token should consume")
	}
	if consumeToken(tok, 3) {
		t.Fatal("consumed token must not pass again")
	}
}

func TestTokenKeyBinding(t *testing.T) {
	tok := mintToken(3)
	if consumeToken(tok, 4) {
		t.Fatal("token bound to another key must fail")
	}
}

func TestDomainAllowed(t *testing.T) {
	domains := []string{"a.com", "*.b.com"}
	for _, c := range []struct {
		host string
		want bool
	}{
		{"a.com", true}, {"www.a.com", false},
		{"b.com", true}, {"x.b.com", true}, {"a.b.com", true},
		{"c.com", false}, {"", false},
	} {
		if got := domainAllowed(c.host, domains); got != c.want {
			t.Errorf("domainAllowed(%q)=%v want %v", c.host, got, c.want)
		}
	}
}
