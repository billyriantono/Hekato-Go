package proxy

import (
	"hekato-go/config"
	"testing"
	"time"
)

func TestAccountSlotsCapInFlight(t *testing.T) {
	old := accountSlotWait
	accountSlotWait = 50 * time.Millisecond
	defer func() { accountSlotWait = old }()

	acc := &config.Account{ID: "slots-test", MaxConcurrency: 2}
	r1, err := acquireAccountSlot(acc)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := acquireAccountSlot(acc)
	if err != nil {
		t.Fatal(err)
	}
	if accountHasFreeSlot(acc) {
		t.Fatal("expected account saturated at 2 in-flight")
	}
	if _, err := acquireAccountSlot(acc); err == nil {
		t.Fatal("third acquire should time out")
	}

	// A queued request starts as soon as a slot frees.
	got := make(chan error, 1)
	go func() {
		r, err := acquireAccountSlot(acc)
		if err == nil {
			r()
		}
		got <- err
	}()
	time.Sleep(10 * time.Millisecond)
	r1()
	if err := <-got; err != nil {
		t.Fatalf("queued acquire failed after release: %v", err)
	}
	r2()
	if !accountHasFreeSlot(acc) {
		t.Fatal("expected free slot after releases")
	}

	if r, err := acquireAccountSlot(&config.Account{ID: "unlimited"}); err != nil {
		t.Fatal(err)
	} else {
		r()
	}
}
