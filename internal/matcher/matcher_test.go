package matcher

import (
	"reflect"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		patternType PatternType
		opts        *Options
		wantErr     bool
	}{
		{
			name:        "valid glob pattern",
			pattern:     "*.txt",
			patternType: Glob,
			wantErr:     false,
		},
		{
			name:        "valid regex pattern",
			pattern:     "^test.*",
			patternType: Regex,
			wantErr:     false,
		},
		{
			name:        "invalid regex pattern",
			pattern:     "[unclosed",
			patternType: Regex,
			wantErr:     true,
		},
		{
			name:        "auto detect glob",
			pattern:     "*.log",
			patternType: Auto,
			wantErr:     false,
		},
		{
			name:        "auto detect regex",
			pattern:     "^test\\d+$",
			patternType: Auto,
			wantErr:     false,
		},
		{
			name:        "case insensitive option",
			pattern:     "test",
			patternType: Glob,
			opts:        &Options{CaseInsensitive: true},
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := New(tt.patternType, tt.pattern, tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && m == nil {
				t.Error("New() returned nil matcher without error")
			}
		})
	}
}

func TestMatcher_Match(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		patternType PatternType
		opts        *Options
		input       string
		want        bool
	}{
		// Glob tests
		{
			name:        "glob exact match",
			pattern:     "test.txt",
			patternType: Glob,
			input:       "test.txt",
			want:        true,
		},
		{
			name:        "glob star wildcard",
			pattern:     "*.txt",
			patternType: Glob,
			input:       "document.txt",
			want:        true,
		},
		{
			name:        "glob question wildcard",
			pattern:     "test?.txt",
			patternType: Glob,
			input:       "test1.txt",
			want:        true,
		},
		{
			name:        "glob character class",
			pattern:     "test[0-9].txt",
			patternType: Glob,
			input:       "test5.txt",
			want:        true,
		},
		{
			name:        "glob no match",
			pattern:     "*.txt",
			patternType: Glob,
			input:       "document.pdf",
			want:        false,
		},
		// Regex tests
		{
			name:        "regex exact match",
			pattern:     "^test$",
			patternType: Regex,
			input:       "test",
			want:        true,
		},
		{
			name:        "regex with digits",
			pattern:     "\\d{3}-\\d{4}",
			patternType: Regex,
			input:       "123-4567",
			want:        true,
		},
		{
			name:        "regex no match",
			pattern:     "^test$",
			patternType: Regex,
			input:       "testing",
			want:        false,
		},
		// Case insensitive tests
		{
			name:        "case insensitive glob",
			pattern:     "TEST.TXT",
			patternType: Glob,
			opts:        &Options{CaseInsensitive: true},
			input:       "test.txt",
			want:        true,
		},
		{
			name:        "case insensitive regex",
			pattern:     "TEST",
			patternType: Regex,
			opts:        &Options{CaseInsensitive: true},
			input:       "test",
			want:        true,
		},
		// Anchored regex tests
		{
			name:        "anchored regex",
			pattern:     "test",
			patternType: Regex,
			opts:        &Options{Anchored: true},
			input:       "test",
			want:        true,
		},
		{
			name:        "anchored regex no match",
			pattern:     "test",
			patternType: Regex,
			opts:        &Options{Anchored: true},
			input:       "testing",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := New(tt.patternType, tt.pattern, tt.opts)
			if err != nil {
				t.Fatalf("Failed to create matcher: %v", err)
			}
			if got := m.Match(tt.input); got != tt.want {
				t.Errorf("Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatcher_MatchAll(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		patternType PatternType
		inputs      []string
		want        []string
	}{
		{
			name:        "glob match multiple",
			pattern:     "*.txt",
			patternType: Glob,
			inputs:      []string{"test.txt", "doc.txt", "file.pdf", "notes.txt"},
			want:        []string{"test.txt", "doc.txt", "notes.txt"},
		},
		{
			name:        "regex match multiple",
			pattern:     "^test",
			patternType: Regex,
			inputs:      []string{"test1", "test2", "example", "testing"},
			want:        []string{"test1", "test2", "testing"},
		},
		{
			name:        "no matches",
			pattern:     "*.exe",
			patternType: Glob,
			inputs:      []string{"test.txt", "doc.pdf", "file.docx"},
			want:        []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := New(tt.patternType, tt.pattern)
			if err != nil {
				t.Fatalf("Failed to create matcher: %v", err)
			}
			got := m.MatchAll(tt.inputs...)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MatchAll() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectPatternType(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    PatternType
	}{
		{"simple glob with star", "*.txt", Glob},
		{"simple glob with question", "test?.log", Glob},
		{"regex with anchor", "^test", Regex},
		{"regex with dollar", "test$", Regex},
		{"regex with digit class", "\\d+", Regex},
		{"regex with group", "(test|example)", Regex},
		{"regex with quantifier", "test{2,4}", Regex},
		{"plain string", "test.txt", Glob},
		{"regex with case flag", "(?i)test", Regex},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := detectPatternType(tt.pattern); got != tt.want {
				t.Errorf("detectPatternType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMultiMatcher(t *testing.T) {
	patterns := []string{"*.txt", "*.log", "*.md"}
	mm, err := NewMultiMatcher(patterns, Glob)
	if err != nil {
		t.Fatalf("Failed to create MultiMatcher: %v", err)
	}

	t.Run("Match", func(t *testing.T) {
		tests := []struct {
			input string
			want  bool
		}{
			{"test.txt", true},
			{"app.log", true},
			{"README.md", true},
			{"script.py", false},
			{"data.csv", false},
		}

		for _, tt := range tests {
			if got := mm.Match(tt.input); got != tt.want {
				t.Errorf("Match(%q) = %v, want %v", tt.input, got, tt.want)
			}
		}
	})

	t.Run("MatchAll", func(t *testing.T) {
		inputs := []string{"test.txt", "app.log", "script.py", "README.md", "data.csv"}
		want := []string{"test.txt", "app.log", "README.md"}
		got := mm.MatchAll(inputs...)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("MatchAll() = %v, want %v", got, want)
		}
	})

	t.Run("MatchAny", func(t *testing.T) {
		if !mm.MatchAny("script.py", "test.txt") {
			t.Error("MatchAny() should return true when at least one matches")
		}
		if mm.MatchAny("script.py", "data.csv") {
			t.Error("MatchAny() should return false when none match")
		}
	})
}

func TestHelperFunctions(t *testing.T) {
	t.Run("MatchFile", func(t *testing.T) {
		matched, err := MatchFile("document.txt", "*.txt")
		if err != nil {
			t.Fatalf("MatchFile() error = %v", err)
		}
		if !matched {
			t.Error("MatchFile() should match *.txt pattern")
		}
	})

	t.Run("FilterStrings", func(t *testing.T) {
		items := []string{"test.txt", "app.log", "README.md", "script.py"}
		filtered, err := FilterStrings(Glob, "*.txt", items...)
		if err != nil {
			t.Fatalf("FilterStrings() error = %v", err)
		}
		want := []string{"test.txt"}
		if !reflect.DeepEqual(filtered, want) {
			t.Errorf("FilterStrings() = %v, want %v", filtered, want)
		}
	})

	t.Run("IsGlobPattern", func(t *testing.T) {
		tests := []struct {
			pattern string
			want    bool
		}{
			{"*.txt", true},
			{"test?.log", true},
			{"file[0-9].txt", true},
			{"plain.txt", false},
			{"no-special", false},
		}

		for _, tt := range tests {
			if got := IsGlobPattern(tt.pattern); got != tt.want {
				t.Errorf("IsGlobPattern(%q) = %v, want %v", tt.pattern, got, tt.want)
			}
		}
	})

	t.Run("GlobToRegex", func(t *testing.T) {
		tests := []struct {
			glob  string
			input string
			match bool
		}{
			{"*.txt", "file.txt", true},
			{"*.txt", "file.pdf", false},
			{"test?.log", "test1.log", true},
			{"test?.log", "test12.log", false},
			{"file[0-9].txt", "file5.txt", true},
			{"file[!0-9].txt", "filea.txt", true},
			{"file[!0-9].txt", "file5.txt", false},
		}

		for _, tt := range tests {
			regex := GlobToRegex(tt.glob)
			m := MustNew(Regex, regex)
			if got := m.Match(tt.input); got != tt.match {
				t.Errorf("GlobToRegex(%q) converted pattern doesn't match %q correctly: got %v, want %v",
					tt.glob, tt.input, got, tt.match)
			}
		}
	})
}

func TestCompilePatterns(t *testing.T) {
	patterns := map[string]PatternType{
		"*.txt":     Glob,
		"^test\\d+": Regex,
		"*.log":     Glob,
	}

	compiled, err := CompilePatterns(patterns)
	if err != nil {
		t.Fatalf("CompilePatterns() error = %v", err)
	}

	if len(compiled) != len(patterns) {
		t.Errorf("CompilePatterns() compiled %d patterns, want %d", len(compiled), len(patterns))
	}

	// Test that compiled patterns work
	if m, ok := compiled["*.txt"]; ok {
		if !m.Match("file.txt") {
			t.Error("Compiled *.txt pattern should match file.txt")
		}
	} else {
		t.Error("*.txt pattern not found in compiled patterns")
	}
}

func TestMustNew(t *testing.T) {
	// Test that MustNew doesn't panic with valid pattern
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("MustNew() panicked with valid pattern: %v", r)
			}
		}()
		m := MustNew(Glob, "*.txt")
		if m == nil {
			t.Error("MustNew() returned nil")
		}
	}()

	// Test that MustNew panics with invalid pattern
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("MustNew() should panic with invalid regex pattern")
			}
		}()
		MustNew(Regex, "[unclosed")
	}()
}

func TestConcurrency(t *testing.T) {
	m := MustNew(Glob, "*.txt")

	// Test concurrent reads
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func(id int) {
			result := m.Match("test.txt")
			if !result {
				t.Errorf("Concurrent match %d failed", id)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 100; i++ {
		<-done
	}
}

func TestMultiMatcherConcurrency(t *testing.T) {
	patterns := []string{"*.txt", "*.log"}
	mm, _ := NewMultiMatcher(patterns, Glob)

	// Test concurrent operations
	done := make(chan bool, 100)

	// Add matchers concurrently
	for i := 0; i < 50; i++ {
		go func() {
			m := MustNew(Glob, "*.md")
			mm.AddMatcher(m)
			done <- true
		}()
	}

	// Match concurrently
	for i := 0; i < 50; i++ {
		go func() {
			_ = mm.Match("test.txt")
			done <- true
		}()
	}

	for i := 0; i < 100; i++ {
		<-done
	}
}
