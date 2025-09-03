package constants_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/agentstation/starmap/pkg/constants"
)

// Example demonstrates using constants for common operations
func Example() {
	// Create directory with standard permissions
	dir := filepath.Join(".", "data")
	if err := os.MkdirAll(dir, constants.DirPermissions); err != nil {
		panic(err)
	}

	// Create file with standard permissions
	file := filepath.Join(dir, "config.yaml")
	data := []byte("config: true")
	if err := os.WriteFile(file, data, constants.FilePermissions); err != nil {
		panic(err)
	}

	fmt.Printf("Created dir with %o permissions\n", constants.DirPermissions)
	fmt.Printf("Created file with %o permissions\n", constants.FilePermissions)
	// Output:
	// Created dir with 755 permissions
	// Created file with 644 permissions
}

// Example_timeouts demonstrates timeout constants
func Example_timeouts() {
	// HTTP client with default timeout
	client := &http.Client{
		Timeout: constants.DefaultHTTPTimeout,
	}
	fmt.Printf("HTTP timeout: %v\n", client.Timeout)

	// Context with operation timeout
	ctx, cancel := context.WithTimeout(
		context.Background(),
		constants.DefaultTimeout,
	)
	defer cancel()

	// Simulated operation
	select {
	case <-time.After(100 * time.Millisecond):
		fmt.Println("Operation completed")
	case <-ctx.Done():
		fmt.Println("Operation timed out")
	}

	// Output:
	// HTTP timeout: 30s
	// Operation completed
}

// Example_gitHubConstants shows GitHub-specific constants
func Example_gitHubConstants() {
	// GitHub URLs
	fmt.Printf("Models.dev URL: %s\n", constants.ModelsDevURL)
	fmt.Printf("Models.dev Git: %s\n", constants.ModelsDevGit)

	// Rate limiting
	fmt.Printf("Max models: %d\n", constants.MaxCatalogModels)
	fmt.Printf("Max providers: %d\n", constants.MaxProviders)

	// Output:
	// Models.dev URL: https://models.dev
	// Models.dev Git: https://github.com/neuralmagic/models.dev.git
	// Max models: 10000
	// Max providers: 100
}

// Example_retryLogic demonstrates using retry constants
func Example_retryLogic() {
	// Exponential backoff with constants
	operation := func() error {
		// Simulated operation that might fail
		return fmt.Errorf("temporary error")
	}

	var lastErr error
	for i := 0; i < constants.MaxRetries; i++ {
		err := operation()
		if err == nil {
			fmt.Println("Success")
			break
		}
		lastErr = err

		if i < constants.MaxRetries-1 {
			// Calculate backoff
			backoff := constants.RetryBackoff * time.Duration(1<<i)
			if backoff > constants.MaxRetryBackoff {
				backoff = constants.MaxRetryBackoff
			}
			fmt.Printf("Retry %d/%d after %v\n", i+1, constants.MaxRetries, backoff)
			time.Sleep(backoff)
		}
	}

	if lastErr != nil {
		fmt.Printf("Failed after %d retries\n", constants.MaxRetries)
	}
}

// Example_bufferSizes shows using buffer size constants
func Example_bufferSizes() {
	// Channel with standard buffer size
	ch := make(chan string, constants.ChannelBufferSize)
	close(ch) // Clean up

	// Write buffer for file operations
	buffer := make([]byte, 0, constants.WriteBufferSize)

	fmt.Printf("Channel buffer: %d\n", constants.ChannelBufferSize)
	fmt.Printf("Write buffer: %d bytes\n", cap(buffer))

	// Output:
	// Channel buffer: 100
	// Write buffer: 4096 bytes
}

// Example_concurrencyLimits demonstrates concurrency constants
func Example_concurrencyLimits() {
	// Worker pool with limited concurrency
	jobs := make(chan int, 100)
	results := make(chan int, 100)

	// Start workers up to max concurrent limit
	for w := 0; w < constants.MaxConcurrentRequests; w++ {
		go func(id int) {
			for job := range jobs {
				// Simulate work
				results <- job * 2
			}
		}(w)
	}

	// Send jobs
	for i := 0; i < 20; i++ {
		jobs <- i
	}
	close(jobs)

	fmt.Printf("Processing with %d workers\n", constants.MaxConcurrentRequests)
	// Output: Processing with 10 workers
}

// Example_rateLimiting shows rate limiting with constants
func Example_rateLimiting() {
	// Create rate limiter
	limiter := time.NewTicker(time.Second / time.Duration(constants.DefaultRateLimit))
	defer limiter.Stop()

	requests := 0
	timeout := time.After(2 * time.Second)

	for {
		select {
		case <-limiter.C:
			requests++
			fmt.Printf("Request %d\n", requests)
			if requests >= 5 {
				return
			}
		case <-timeout:
			fmt.Printf("Made %d requests in 2 seconds\n", requests)
			return
		}
	}
}

// Example_updateInterval demonstrates update interval usage
func Example_updateInterval() {
	// Auto-update ticker
	ticker := time.NewTicker(constants.DefaultUpdateInterval)
	defer ticker.Stop()

	// Simulated update check
	updates := 0
	timeout := time.After(3 * time.Second)

	for {
		select {
		case <-ticker.C:
			updates++
			fmt.Printf("Checking for updates... (check #%d)\n", updates)
		case <-timeout:
			fmt.Printf("Performed %d update checks\n", updates)
			return
		}
	}
}

// Example_contextTimeouts shows different context timeout scenarios
func Example_contextTimeouts() {
	// Short operation
	_, shortCancel := context.WithTimeout(
		context.Background(),
		constants.DefaultTimeout,
	)
	defer shortCancel()

	// Long operation
	_, longCancel := context.WithTimeout(
		context.Background(),
		constants.UpdateContextTimeout,
	)
	defer longCancel()

	// Sync operation
	_, syncCancel := context.WithTimeout(
		context.Background(),
		constants.SyncTimeout,
	)
	defer syncCancel()

	fmt.Printf("Default timeout: %v\n", constants.DefaultTimeout)
	fmt.Printf("Update timeout: %v\n", constants.UpdateContextTimeout)
	fmt.Printf("Sync timeout: %v\n", constants.SyncTimeout)

	// Output:
	// Default timeout: 10s
	// Update timeout: 5m0s
	// Sync timeout: 30m0s
}
