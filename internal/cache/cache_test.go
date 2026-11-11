package cache

import (
	"testing"
	"time"
)

func TestCacheExpiresAtTTLBoundary(t *testing.T) {
	now := time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC)
	cache := New[string, string](time.Minute, 0)
	cache.now = func() time.Time {
		return now
	}
	key := Key[string]{ID: "42"}
	cache.Set(key, "Ada", 0)

	now = now.Add(time.Minute - time.Nanosecond)
	if value, found, _ := cache.Get(key); !found || value != "Ada" {
		t.Fatalf("expired too early: %q, found=%v", value, found)
	}

	now = now.Add(time.Nanosecond)
	if _, found, _ := cache.Get(key); found {
		t.Fatal("entry did not expire at TTL boundary")
	}

	if len(cache.entries) != 0 {
		t.Fatal("expired entry was not removed")
	}
}

func TestCacheEvictsLeastRecentlyUsedEntry(t *testing.T) {
	cache := New[string, string](time.Minute, 2)
	first := Key[string]{ID: "first"}
	second := Key[string]{ID: "second"}
	third := Key[string]{ID: "third"}
	cache.Set(first, "1", 0)
	cache.Set(second, "2", 0)
	cache.Get(first)
	cache.Set(third, "3", 0)

	if _, found, _ := cache.Get(second); found {
		t.Fatal("least recently used entry was not evicted")
	}

	for _, key := range []Key[string]{
		first,
		third,
	} {
		if _, found, _ := cache.Get(key); !found {
			t.Fatalf("unexpectedly evicted %q", key.ID)
		}
	}

	if len(cache.entries) != 2 || cache.recent.Len() != 2 {
		t.Fatal("cache exceeded its capacity")
	}
}
