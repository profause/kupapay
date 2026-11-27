package cache

import (
	"context"
	"testing"
	"time"
)

func TestMemoryCacheJSONRoundTrip(t *testing.T) {
	mc, err := NewMemoryCache(MemoryCacheConfig{DefaultTTL: time.Minute})
	if err != nil {
		t.Fatalf("new memory cache: %v", err)
	}
	defer mc.Close()

	ctx := context.Background()
	type payload struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	want := payload{Name: "Ada", Age: 32}
	if err := mc.SetJSON(ctx, "user:1", want, time.Minute); err != nil {
		t.Fatalf("set json: %v", err)
	}

	var got payload
	if err := mc.GetJSON(ctx, "user:1", &got); err != nil {
		t.Fatalf("get json: %v", err)
	}

	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestMemoryCacheSetIfAbsent(t *testing.T) {
	mc, err := NewMemoryCache(MemoryCacheConfig{DefaultTTL: time.Minute})
	if err != nil {
		t.Fatalf("new memory cache: %v", err)
	}
	defer mc.Close()

	ctx := context.Background()

	ok, err := mc.SetIfAbsent(ctx, "guard", "first", time.Minute)
	if err != nil {
		t.Fatalf("first set-if-absent: %v", err)
	}
	if !ok {
		t.Fatal("expected first set-if-absent to succeed")
	}

	ok, err = mc.SetIfAbsent(ctx, "guard", "second", time.Minute)
	if err != nil {
		t.Fatalf("second set-if-absent: %v", err)
	}
	if ok {
		t.Fatal("expected second set-if-absent to fail")
	}

	got, err := mc.Get(ctx, "guard")
	if err != nil {
		t.Fatalf("get guard: %v", err)
	}
	if got != "first" {
		t.Fatalf("got %q, want %q", got, "first")
	}
}
