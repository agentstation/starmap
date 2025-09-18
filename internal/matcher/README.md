# Pattern Matcher

A high-performance Go package for pattern matching that supports both glob and regex patterns with a unified interface. Perfect for file filtering, string matching, and batch operations.

## Features

- 🎯 **Dual Pattern Support**: Seamlessly use glob (`*.txt`) or regex (`^test.*$`) patterns
- 🚀 **High Performance**: Optimized for speed with compile-time pattern optimization
- 🔄 **Auto-Detection**: Automatically detect pattern type when needed
- 🎛️ **Flexible Options**: Case-insensitive matching, anchored patterns, and more
- 🔧 **Thread-Safe**: Safe for concurrent use in goroutines
- 📦 **Batch Operations**: Efficiently match multiple inputs at once
- 🛠️ **Helper Functions**: Convenient utilities for common use cases

## Installation

```bash
go get github.com/yourusername/matcher
```

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/yourusername/matcher"
)

func main() {
    // Create a glob matcher
    m, err := matcher.New(matcher.Glob, "*.txt")
    if err != nil {
        panic(err)
    }
    
    // Check if a file matches
    if m.Match("document.txt") {
        fmt.Println("It's a text file!")
    }
    
    // Filter a list of files
    textFiles := m.MatchAll("doc.txt", "image.png", "notes.txt", "data.csv")
    fmt.Println("Text files:", textFiles)
    // Output: Text files: [doc.txt notes.txt]
}
```

## Usage Examples

### Basic Pattern Matching

#### Glob Patterns

```go
// Simple glob pattern
m := matcher.MustNew(matcher.Glob, "*.log")
fmt.Println(m.Match("app.log"))        // true
fmt.Println(m.Match("app.txt"))        // false

// Glob with character classes
m = matcher.MustNew(matcher.Glob, "file[0-9].txt")
fmt.Println(m.Match("file5.txt"))      // true
fmt.Println(m.Match("fileA.txt"))      // false

// Glob with question mark wildcard
m = matcher.MustNew(matcher.Glob, "test?.log")
fmt.Println(m.Match("test1.log"))      // true
fmt.Println(m.Match("test12.log"))     // false
```

#### Regex Patterns

```go
// Simple regex
m := matcher.MustNew(matcher.Regex, "^test\\d+$")
fmt.Println(m.Match("test123"))        // true
fmt.Println(m.Match("test"))           // false

// Complex regex with groups
m = matcher.MustNew(matcher.Regex, "(error|warning|info):\\s+.*")
fmt.Println(m.Match("error: file not found"))    // true
fmt.Println(m.Match("debug: starting"))          // false
```

### Auto-Detection

```go
// Let the package detect the pattern type
patterns := []string{
    "*.txt",           // Detected as Glob
    "^test.*$",        // Detected as Regex
    "file[0-9].log",   // Detected as Glob
    "\\d{3}-\\d{4}",   // Detected as Regex
}

for _, pattern := range patterns {
    m, _ := matcher.New(matcher.Auto, pattern)
    fmt.Printf("Pattern '%s' detected as: %s\n", pattern, m.Type())
}
```

### Options

```go
// Case-insensitive matching
opts := &matcher.Options{
    CaseInsensitive: true,
}
m, _ := matcher.New(matcher.Glob, "*.TXT", opts)
fmt.Println(m.Match("document.txt"))   // true

// Anchored regex (automatically adds ^ and $ if not present)
opts = &matcher.Options{
    Anchored: true,
}
m, _ = matcher.New(matcher.Regex, "test", opts)
fmt.Println(m.Match("test"))           // true
fmt.Println(m.Match("testing"))        // false
```

### Multi-Pattern Matching

```go
// Match against multiple patterns
patterns := []string{"*.txt", "*.md", "*.log"}
mm, err := matcher.NewMultiMatcher(patterns, matcher.Glob)
if err != nil {
    panic(err)
}

// Check if any pattern matches
fmt.Println(mm.Match("README.md"))     // true
fmt.Println(mm.Match("script.py"))     // false

// Filter files
matched := mm.MatchAll("doc.txt", "README.md", "app.log", "image.png", "script.py")
fmt.Println(matched)  // [doc.txt README.md app.log]

// Check if any input matches any pattern
fmt.Println(mm.MatchAny("test.py", "doc.txt"))  // true
```

### Helper Functions

```go
// Quick file matching
matched, _ := matcher.MatchFile("document.txt", "*.txt")
fmt.Println(matched)  // true

// Filter strings
filtered, _ := matcher.FilterStrings(matcher.Glob, "*.txt", "test.txt", "app.log", "README.md", "script.py")
fmt.Println(filtered)  // [test.txt]

// Check pattern type
fmt.Println(matcher.IsGlobPattern("*.txt"))      // true
fmt.Println(matcher.IsRegexPattern("^test.*$"))  // true

// Convert glob to regex
regex := matcher.GlobToRegex("*.txt")
fmt.Println(regex)  // ^.*\.txt$
```

### Batch Pattern Compilation

```go
// Pre-compile multiple patterns for efficiency
patterns := map[string]matcher.PatternType{
    "*.txt":        matcher.Glob,
    "*.log":        matcher.Glob,
    "^ERROR:.*":    matcher.Regex,
    "^WARNING:.*":  matcher.Regex,
}

compiled, err := matcher.CompilePatterns(patterns)
if err != nil {
    panic(err)
}

// Use compiled patterns
for pattern, m := range compiled {
    fmt.Printf("Pattern %s matches 'ERROR: failed': %v\n", 
        pattern, m.Match("ERROR: failed"))
}
```

### Real-World Examples

#### Log File Filtering

```go
func filterLogFiles(directory string) ([]string, error) {
    // Create a multi-matcher for log-related files
    patterns := []string{
        "*.log",
        "*.log.[0-9]",
        "*.err",
        "error-*.txt",
    }
    
    mm, err := matcher.NewMultiMatcher(patterns, matcher.Glob)
    if err != nil {
        return nil, err
    }
    
    files, err := os.ReadDir(directory)
    if err != nil {
        return nil, err
    }
    
    var logFiles []string
    for _, file := range files {
        if !file.IsDir() && mm.Match(file.Name()) {
            logFiles = append(logFiles, file.Name())
        }
    }
    
    return logFiles, nil
}
```

#### Configuration File Discovery

```go
func findConfigFiles(root string) ([]string, error) {
    configMatcher, _ := matcher.NewMultiMatcher([]string{
        "*.conf",
        "*.config",
        "*.yml",
        "*.yaml",
        "*.json",
        "*.toml",
    }, matcher.Glob)
    
    var configs []string
    
    err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        
        if !info.IsDir() && configMatcher.Match(filepath.Base(path)) {
            configs = append(configs, path)
        }
        
        return nil
    })
    
    return configs, err
}
```

#### Log Level Filtering

```go
func filterLogsByLevel(levels []string, logs ...string) []string {
    // Build regex patterns for log levels
    patterns := make([]string, len(levels))
    for i, level := range levels {
        patterns[i] = fmt.Sprintf("(?i)^\\[%s\\]", level)
    }
    
    mm, _ := matcher.NewMultiMatcher(patterns, matcher.Regex)
    return mm.MatchAll(logs...)
}

// Usage
errors := filterLogsByLevel([]string{"ERROR", "WARNING"}, 
    "[ERROR] Failed to connect",
    "[INFO] Server started", 
    "[DEBUG] Processing request",
    "[WARNING] Memory usage high")
// Returns: ["[ERROR] Failed to connect", "[WARNING] Memory usage high"]
```

## Performance

The package is optimized for high performance with minimal allocations:

```
BenchmarkGlobMatch-8                 20000000        65.3 ns/op       0 B/op       0 allocs/op
BenchmarkRegexMatch-8                 5000000       285.2 ns/op       0 B/op       0 allocs/op
BenchmarkGlobMatchComplex-8          10000000       142.7 ns/op       0 B/op       0 allocs/op
BenchmarkRegexMatchComplex-8          3000000       412.8 ns/op       0 B/op       0 allocs/op
BenchmarkMultiMatcherGlob-8           5000000       234.1 ns/op       0 B/op       0 allocs/op
BenchmarkGlobMatchParallel-8         50000000        24.8 ns/op       0 B/op       0 allocs/op
```

## API Reference

### Types

- `PatternType`: Enum for pattern types (Glob, Regex, Auto)
- `Matcher`: Main interface for pattern matching
- `MultiMatcher`: Handles multiple patterns simultaneously
- `Options`: Configuration for matcher behavior

### Main Functions

- `New(patternType PatternType, pattern string, opts ...*Options) (Matcher, error)`
- `MustNew(patternType PatternType, pattern string, opts ...*Options) Matcher`
- `NewMultiMatcher(patterns []string, patternType PatternType, opts ...*Options) (*MultiMatcher, error)`

### Helper Functions

- `MatchFile(path, pattern string) (bool, error)`
- `FilterStrings(patternType PatternType, pattern string, items ...string) ([]string, error)`
- `IsGlobPattern(pattern string) bool`
- `IsRegexPattern(pattern string) bool`
- `GlobToRegex(glob string) string`
- `CompilePatterns(patterns map[string]PatternType, opts ...*Options) (map[string]Matcher, error)`

## Thread Safety

All matchers are thread-safe and can be safely used concurrently:

```go
m := matcher.MustNew("*.txt", matcher.Glob)

var wg sync.WaitGroup
for i := 0; i < 100; i++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        filename := fmt.Sprintf("file%d.txt", id)
        if m.Match(filename) {
            // Process matching file
        }
    }(i)
}
wg.Wait()
```

## Testing

Run tests with coverage:

```bash
go test -v -cover ./...
```

Run benchmarks:

```bash
go test -bench=. -benchmem ./...
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see LICENSE file for details