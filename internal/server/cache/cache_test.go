package cache

import (
	"sync"
	"testing"
	"time"
)

// TestCache_New tests cache creation.
func TestCache_New(t *testing.T) {
	c := New(5*time.Minute, 10*time.Minute)
	if c == nil {
		t.Fatal("New() returned nil")
	}
	if c.store == nil {
		t.Error("cache store not initialized")
	}
}

// TestCache_BasicOperations tests Get, Set, and Delete.
func TestCache_BasicOperations(t *testing.T) {
	c := New(5*time.Minute, 10*time.Minute)

	t.Run("Set and Get", func(t *testing.T) {
		c.Set("key1", "value1")

		val, found := c.Get("key1")
		if !found {
			t.Error("expected key1 to be found")
		}
		if val != "value1" {
			t.Errorf("expected value1, got %v", val)
		}
	})

	t.Run("Get non-existent key", func(t *testing.T) {
		_, found := c.Get("nonexistent")
		if found {
			t.Error("expected nonexistent key to not be found")
		}
	})

	t.Run("Set and Delete", func(t *testing.T) {
		c.Set("key2", "value2")
		c.Delete("key2")

		_, found := c.Get("key2")
		if found {
			t.Error("expected key2 to be deleted")
		}
	})

	t.Run("Delete non-existent key", func(t *testing.T) {
		// Should not panic
		c.Delete("nonexistent")
	})
}

// TestCache_SetWithTTL tests custom TTL.
func TestCache_SetWithTTL(t *testing.T) {
	c := New(5*time.Minute, 10*time.Minute)

	// Set with very short TTL
	c.SetWithTTL("expiring", "value", 50*time.Millisecond)

	// Should exist immediately
	_, found := c.Get("expiring")
	if !found {
		t.Error("expected key to exist immediately")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired
	_, found = c.Get("expiring")
	if found {
		t.Error("expected key to be expired")
	}
}

// TestCache_Clear tests clearing all items.
func TestCache_Clear(t *testing.T) {
	c := New(5*time.Minute, 10*time.Minute)

	// Add multiple items
	c.Set("key1", "value1")
	c.Set("key2", "value2")
	c.Set("key3", "value3")

	if count := c.ItemCount(); count != 3 {
		t.Errorf("expected 3 items, got %d", count)
	}

	// Clear cache
	c.Clear()

	if count := c.ItemCount(); count != 0 {
		t.Errorf("expected 0 items after clear, got %d", count)
	}

	// Verify items are gone
	_, found := c.Get("key1")
	if found {
		t.Error("expected key1 to be cleared")
	}
}

// TestCache_ItemCount tests item counting.
func TestCache_ItemCount(t *testing.T) {
	c := New(5*time.Minute, 10*time.Minute)

	tests := []struct {
		name     string
		setup    func()
		expected int
	}{
		{
			name:     "empty cache",
			setup:    func() {},
			expected: 0,
		},
		{
			name: "one item",
			setup: func() {
				c.Set("key1", "value1")
			},
			expected: 1,
		},
		{
			name: "multiple items",
			setup: func() {
				c.Set("key2", "value2")
				c.Set("key3", "value3")
			},
			expected: 3,
		},
		{
			name: "after deletion",
			setup: func() {
				c.Delete("key1")
			},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			count := c.ItemCount()
			if count != tt.expected {
				t.Errorf("expected %d items, got %d", tt.expected, count)
			}
		})
	}
}

// TestCache_GetStats tests statistics retrieval.
func TestCache_GetStats(t *testing.T) {
	c := New(5*time.Minute, 10*time.Minute)

	// Add some items
	c.Set("key1", "value1")
	c.Set("key2", "value2")

	stats := c.GetStats()
	if stats.ItemCount != 2 {
		t.Errorf("expected ItemCount=2, got %d", stats.ItemCount)
	}
}

// TestCache_ConcurrentAccess tests thread-safety with concurrent operations.
func TestCache_ConcurrentAccess(t *testing.T) {
	c := New(5*time.Minute, 10*time.Minute)

	const numGoroutines = 100
	const numOperations = 100

	var wg sync.WaitGroup

	// Concurrent writes
	t.Run("concurrent writes", func(t *testing.T) {
		wg.Add(numGoroutines)
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					key := "key-" + string(rune(id)) + "-" + string(rune(j))
					c.Set(key, id*numOperations+j)
				}
			}(i)
		}
		wg.Wait()
	})

	// Concurrent reads
	t.Run("concurrent reads", func(t *testing.T) {
		wg.Add(numGoroutines)
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					key := "key-" + string(rune(id)) + "-" + string(rune(j))
					c.Get(key)
				}
			}(i)
		}
		wg.Wait()
	})

	// Mixed operations
	t.Run("mixed operations", func(t *testing.T) {
		wg.Add(numGoroutines * 3)

		// Writers
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					c.Set("mixed-"+string(rune(id)), j)
				}
			}(i)
		}

		// Readers
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					c.Get("mixed-" + string(rune(id)))
				}
			}(i)
		}

		// Deleters
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					c.Delete("mixed-" + string(rune(id)))
				}
			}(i)
		}

		wg.Wait()
	})

	// Should not panic - test passes if we get here
}

// TestCache_ComplexTypes tests caching complex data types.
func TestCache_ComplexTypes(t *testing.T) {
	c := New(5*time.Minute, 10*time.Minute)

	type TestStruct struct {
		Name  string
		Count int
		Tags  []string
	}

	tests := []struct {
		name  string
		key   string
		value any
	}{
		{
			name:  "string",
			key:   "str",
			value: "hello",
		},
		{
			name:  "int",
			key:   "int",
			value: 42,
		},
		{
			name:  "slice",
			key:   "slice",
			value: []string{"a", "b", "c"},
		},
		{
			name:  "map",
			key:   "map",
			value: map[string]int{"one": 1, "two": 2},
		},
		{
			name: "struct",
			key:  "struct",
			value: TestStruct{
				Name:  "test",
				Count: 123,
				Tags:  []string{"tag1", "tag2"},
			},
		},
		{
			name: "pointer",
			key:  "ptr",
			value: &TestStruct{
				Name:  "pointer-test",
				Count: 456,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.Set(tt.key, tt.value)

			val, found := c.Get(tt.key)
			if !found {
				t.Errorf("expected key %q to be found", tt.key)
			}

			// Type assertion depends on the type
			// Just verify we got something back
			if val == nil {
				t.Errorf("expected non-nil value for key %q", tt.key)
			}
		})
	}
}

// TestCache_Overwrite tests overwriting existing keys.
func TestCache_Overwrite(t *testing.T) {
	c := New(5*time.Minute, 10*time.Minute)

	// Set initial value
	c.Set("key", "value1")

	val, _ := c.Get("key")
	if val != "value1" {
		t.Errorf("expected value1, got %v", val)
	}

	// Overwrite with new value
	c.Set("key", "value2")

	val, _ = c.Get("key")
	if val != "value2" {
		t.Errorf("expected value2, got %v", val)
	}

	// Verify only one item in cache
	if count := c.ItemCount(); count != 1 {
		t.Errorf("expected 1 item, got %d", count)
	}
}

// TestCache_DefaultExpiration tests default TTL behavior.
func TestCache_DefaultExpiration(t *testing.T) {
	// Create cache with 100ms default TTL
	c := New(100*time.Millisecond, 200*time.Millisecond)

	c.Set("key", "value")

	// Should exist immediately
	_, found := c.Get("key")
	if !found {
		t.Error("expected key to exist immediately")
	}

	// Wait for default expiration
	time.Sleep(150 * time.Millisecond)

	// Should be expired
	_, found = c.Get("key")
	if found {
		t.Error("expected key to be expired after default TTL")
	}
}
