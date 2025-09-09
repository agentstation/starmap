package inspect

import (
	"io/fs"
	"testing"
	"time"
)

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "."},
		{".", "."},
		{"/", "."},
		{"catalog", "catalog"},
		{"/catalog", "catalog"},
		{"catalog/providers", "catalog/providers"},
		{"/catalog/providers/", "catalog/providers"},
		{"catalog/../providers", "providers"},
		{"./catalog", "catalog"},
	}
	
	for _, test := range tests {
		result := NormalizePath(test.input)
		if result != test.expected {
			t.Errorf("NormalizePath(%q) = %q, want %q", test.input, result, test.expected)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{266911, "260.7 KB"}, // Size of our api.json
	}
	
	for _, test := range tests {
		result := FormatBytes(test.input)
		if result != test.expected {
			t.Errorf("FormatBytes(%d) = %q, want %q", test.input, result, test.expected)
		}
	}
}

func TestFormatTime(t *testing.T) {
	testTime := time.Date(2024, 12, 25, 15, 30, 45, 0, time.UTC)
	result := FormatTime(testTime)
	expected := "Dec 25 15:30"
	
	if result != expected {
		t.Errorf("FormatTime(%v) = %q, want %q", testTime, result, expected)
	}
}

func TestIsHidden(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"file.txt", false},
		{".hidden", true},
		{".git", true},
		{"normal.yaml", false},
		{"..", true},
		{".", true},
	}
	
	for _, test := range tests {
		result := IsHidden(test.input)
		if result != test.expected {
			t.Errorf("IsHidden(%q) = %v, want %v", test.input, result, test.expected)
		}
	}
}

func TestFormatMode(t *testing.T) {
	tests := []struct {
		mode     fs.FileMode
		expected string
	}{
		{fs.ModeDir, "dr--r--r--"},
		{0, "-r--r--r--"},
	}
	
	for _, test := range tests {
		result := FormatMode(test.mode)
		if result != test.expected {
			t.Errorf("FormatMode(%v) = %q, want %q", test.mode, result, test.expected)
		}
	}
}