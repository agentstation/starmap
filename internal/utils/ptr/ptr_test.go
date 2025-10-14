package ptr

import "testing"

func TestTo(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		s := "test"
		ptr := To(s)
		if ptr == nil {
			t.Fatal("Expected non-nil pointer")
		}
		if *ptr != s {
			t.Errorf("Expected %q, got %q", s, *ptr)
		}
		// Verify it's a different address
		if ptr == &s {
			t.Error("Expected different address")
		}
	})

	t.Run("int", func(t *testing.T) {
		i := 42
		ptr := To(i)
		if ptr == nil {
			t.Fatal("Expected non-nil pointer")
		}
		if *ptr != i {
			t.Errorf("Expected %d, got %d", i, *ptr)
		}
	})

	t.Run("custom type", func(t *testing.T) {
		type CustomID string
		id := CustomID("custom-123")
		ptr := To(id)
		if ptr == nil {
			t.Fatal("Expected non-nil pointer")
		}
		if *ptr != id {
			t.Errorf("Expected %q, got %q", id, *ptr)
		}
	})
}

func TestString(t *testing.T) {
	s := "hello world"
	ptr := String(s)
	if ptr == nil {
		t.Fatal("Expected non-nil pointer")
	}
	if *ptr != s {
		t.Errorf("Expected %q, got %q", s, *ptr)
	}
}

func TestBool(t *testing.T) {
	b := true
	ptr := Bool(b)
	if ptr == nil {
		t.Fatal("Expected non-nil pointer")
	}
	if *ptr != b {
		t.Errorf("Expected %t, got %t", b, *ptr)
	}
}

func TestInt(t *testing.T) {
	i := 123
	ptr := Int(i)
	if ptr == nil {
		t.Fatal("Expected non-nil pointer")
	}
	if *ptr != i {
		t.Errorf("Expected %d, got %d", i, *ptr)
	}
}

func TestInt64(t *testing.T) {
	i := int64(9876543210)
	ptr := Int64(i)
	if ptr == nil {
		t.Fatal("Expected non-nil pointer")
	}
	if *ptr != i {
		t.Errorf("Expected %d, got %d", i, *ptr)
	}
}

func TestFloat64(t *testing.T) {
	f := 3.14159
	ptr := Float64(f)
	if ptr == nil {
		t.Fatal("Expected non-nil pointer")
	}
	if *ptr != f {
		t.Errorf("Expected %f, got %f", f, *ptr)
	}
}

func TestMutationIndependence(t *testing.T) {
	original := "original"
	ptr := String(original)

	// Modify through pointer
	*ptr = "modified"

	// Original should be unchanged
	if original != "original" {
		t.Error("Original value should not be affected by pointer mutation")
	}
	if *ptr != "modified" {
		t.Error("Pointer value should be modified")
	}
}
