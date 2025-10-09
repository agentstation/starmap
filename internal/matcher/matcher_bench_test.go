package matcher

import (
	"fmt"
	"testing"
)

// Benchmark data
var (
	benchPatterns = []string{
		"*.txt",
		"test*.log",
		"file[0-9][0-9].dat",
		"document_????.pdf",
		"report_*_2024.xlsx",
	}

	benchRegexPatterns = []string{
		"^.*\\.txt$",
		"^test.*\\.log$",
		"^file\\d{2}\\.dat$",
		"^document_.{4}\\.pdf$",
		"^report_.*_2024\\.xlsx$",
	}

	benchInputs = []string{
		"file.txt",
		"test_app.log",
		"file42.dat",
		"document_2024.pdf",
		"report_january_2024.xlsx",
		"image.png",
		"video.mp4",
		"archive.zip",
		"script.py",
		"data.csv",
	}
)

// Benchmarks for single pattern matching

func BenchmarkGlobMatch(b *testing.B) {
	m := MustNew(Glob, "*.txt")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = m.Match("document.txt")
	}
}

func BenchmarkRegexMatch(b *testing.B) {
	m := MustNew(Regex, "^.*\\.txt$")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = m.Match("document.txt")
	}
}

func BenchmarkGlobMatchComplex(b *testing.B) {
	m := MustNew(Glob, "report_*_[0-9][0-9][0-9][0-9].xlsx")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = m.Match("report_january_2024.xlsx")
	}
}

func BenchmarkRegexMatchComplex(b *testing.B) {
	m := MustNew(Regex, "^report_.*_\\d{4}\\.xlsx$")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = m.Match("report_january_2024.xlsx")
	}
}

// Benchmarks for case-insensitive matching

func BenchmarkGlobMatchCaseInsensitive(b *testing.B) {
	m := MustNew(Glob, "*.TXT", &Options{CaseInsensitive: true})
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = m.Match("document.txt")
	}
}

func BenchmarkRegexMatchCaseInsensitive(b *testing.B) {
	m := MustNew(Regex, ".*\\.TXT", &Options{CaseInsensitive: true})
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = m.Match("document.txt")
	}
}

// Benchmarks for batch matching

func BenchmarkGlobMatchAll(b *testing.B) {
	m := MustNew(Glob, "*.txt")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = m.MatchAll(benchInputs...)
	}
}

func BenchmarkRegexMatchAll(b *testing.B) {
	m := MustNew(Regex, "^.*\\.txt$")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = m.MatchAll(benchInputs...)
	}
}

// Benchmarks for MultiMatcher

func BenchmarkMultiMatcherGlob(b *testing.B) {
	mm, _ := NewMultiMatcher(benchPatterns, Glob)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = mm.Match("report_january_2024.xlsx")
	}
}

func BenchmarkMultiMatcherRegex(b *testing.B) {
	mm, _ := NewMultiMatcher(benchRegexPatterns, Regex)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = mm.Match("report_january_2024.xlsx")
	}
}

func BenchmarkMultiMatcherMatchAll(b *testing.B) {
	mm, _ := NewMultiMatcher(benchPatterns, Glob)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = mm.MatchAll(benchInputs...)
	}
}

// Benchmarks for pattern compilation

func BenchmarkNewGlob(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = New(Glob, "*.txt")
	}
}

func BenchmarkNewRegex(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = New(Regex, "^.*\\.txt$")
	}
}

func BenchmarkNewAuto(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = New(Auto, "*.txt")
	}
}

// Benchmarks for helper functions

func BenchmarkGlobToRegex(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = GlobToRegex("report_*_[0-9][0-9][0-9][0-9].xlsx")
	}
}

func BenchmarkIsGlobPattern(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = IsGlobPattern("*.txt")
	}
}

func BenchmarkIsRegexPattern(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = IsRegexPattern("^test.*$")
	}
}

func BenchmarkFilterStrings(b *testing.B) {
	inputs := make([]string, 100)
	for i := range inputs {
		inputs[i] = fmt.Sprintf("file%d.txt", i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = FilterStrings(Glob, "file[0-9]*.txt", inputs...)
	}
}

// Comparative benchmarks for different pattern complexities

func BenchmarkPatternComplexity(b *testing.B) {
	testCases := []struct {
		name    string
		pattern string
		pType   PatternType
	}{
		{"Glob-Simple", "*.txt", Glob},
		{"Glob-Medium", "test_*.log", Glob},
		{"Glob-Complex", "file_[0-9][0-9]_*_[a-z].dat", Glob},
		{"Regex-Simple", "^.*\\.txt$", Regex},
		{"Regex-Medium", "^test_.*\\.log$", Regex},
		{"Regex-Complex", "^file_\\d{2}_.*_[a-z]\\.dat$", Regex},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			m := MustNew(tc.pType, tc.pattern)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_ = m.Match("file_42_test_x.dat")
			}
		})
	}
}

// Parallel benchmarks

func BenchmarkGlobMatchParallel(b *testing.B) {
	m := MustNew(Glob, "*.txt")
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = m.Match("document.txt")
		}
	})
}

func BenchmarkRegexMatchParallel(b *testing.B) {
	m := MustNew(Regex, "^.*\\.txt$")
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = m.Match("document.txt")
		}
	})
}

func BenchmarkMultiMatcherParallel(b *testing.B) {
	mm, _ := NewMultiMatcher(benchPatterns, Glob)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = mm.Match("test.txt")
		}
	})
}

// Memory allocation benchmarks

func BenchmarkGlobMatchAllocs(b *testing.B) {
	m := MustNew(Glob, "*.txt")
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = m.Match("document.txt")
	}
}

func BenchmarkRegexMatchAllocs(b *testing.B) {
	m := MustNew(Regex, "^.*\\.txt$")
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = m.Match("document.txt")
	}
}

func BenchmarkMatchAllAllocs(b *testing.B) {
	m := MustNew(Glob, "*.txt")
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = m.MatchAll(benchInputs...)
	}
}
