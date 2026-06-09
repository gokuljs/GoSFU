package roomquota

import (
	"errors"
	"testing"
)

func TestStoreResetAndGet(t *testing.T) {
	store := NewStore(0)

	state := store.Reset("room-a")
	if state.Limit != DefaultLimit {
		t.Fatalf("limit = %d, want %d", state.Limit, DefaultLimit)
	}
	if state.Used != 0 || state.ActiveTurn != 0 || state.Exhausted {
		t.Fatalf("unexpected initial state: %+v", state)
	}
}

func TestStoreStartAndCompleteTurn(t *testing.T) {
	store := NewStore(2)
	store.Reset("room-a")

	state, err := store.StartTurn("room-a", 1)
	if err != nil {
		t.Fatalf("StartTurn returned error: %v", err)
	}
	if state.ActiveTurn != 1 || state.Used != 0 {
		t.Fatalf("state after start = %+v, want active turn 1 and used 0", state)
	}

	state, err = store.CompleteTurn("room-a", 1)
	if err != nil {
		t.Fatalf("CompleteTurn returned error: %v", err)
	}
	if state.ActiveTurn != 0 || state.Used != 1 || state.Exhausted {
		t.Fatalf("state after complete = %+v, want used 1 and not exhausted", state)
	}
}

func TestStoreDoesNotCountGreetingOrDuplicateCompletions(t *testing.T) {
	store := NewStore(2)
	store.Reset("room-a")

	if state, err := store.CompleteTurn("room-a", 0); err != nil || state.Used != 0 {
		t.Fatalf("greeting complete state = %+v, err = %v; want free", state, err)
	}

	if _, err := store.CompleteTurn("room-a", 1); err != nil {
		t.Fatalf("first completion returned error: %v", err)
	}
	state, err := store.CompleteTurn("room-a", 1)
	if err != nil {
		t.Fatalf("duplicate completion returned error: %v", err)
	}
	if state.Used != 1 {
		t.Fatalf("duplicate completion used = %d, want 1", state.Used)
	}
}

func TestStoreExhaustionBlocksFutureTurns(t *testing.T) {
	store := NewStore(1)
	store.Reset("room-a")

	if _, err := store.StartTurn("room-a", 1); err != nil {
		t.Fatalf("StartTurn returned error: %v", err)
	}
	state, err := store.CompleteTurn("room-a", 1)
	if !errors.Is(err, ErrExhausted) {
		t.Fatalf("CompleteTurn err = %v, want ErrExhausted", err)
	}
	if !state.Exhausted || state.Used != 1 {
		t.Fatalf("state after exhaustion = %+v, want exhausted with used 1", state)
	}

	state, err = store.StartTurn("room-a", 2)
	if !errors.Is(err, ErrExhausted) {
		t.Fatalf("StartTurn after exhaustion err = %v, want ErrExhausted", err)
	}
	if state.ActiveTurn != 0 || !state.Exhausted {
		t.Fatalf("blocked state = %+v, want no active turn and exhausted", state)
	}
	if !store.IsExhausted("room-a") {
		t.Fatal("IsExhausted returned false, want true")
	}
}
