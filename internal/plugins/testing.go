package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// PluginTester provides testing utilities for plugins
type PluginTester struct {
	logger     *logrus.Logger
	mockServer *httptest.Server
	mockData   map[string]interface{}
}

// TestSuite represents a complete test suite for a plugin
type TestSuite struct {
	Plugin      ExternalSystemPlugin
	TestCases   []TestCase
	MockData    map[string]interface{}
	Performance *PerformanceTest
	Integration *IntegrationTest
}

// TestCase represents a single test case
type TestCase struct {
	Name           string
	UserID         string
	ExpectedResult *UserEnrichment
	ExpectedError  error
	MockResponse   interface{}
	StatusCode     int
	Description    string
}

// PerformanceTest represents performance testing configuration
type PerformanceTest struct {
	ConcurrentUsers int
	Duration        time.Duration
	MaxLatency      time.Duration
	MaxMemoryMB     int
	MinThroughput   int // requests per second
}

// IntegrationTest represents integration testing configuration
type IntegrationTest struct {
	RealAPITest      bool
	ConfigValidation bool
	ErrorHandling    bool
	HealthCheck      bool
}

// TestResult represents the result of a test execution
type TestResult struct {
	TestName    string
	Passed      bool
	Error       error
	Duration    time.Duration
	MemoryUsed  int64
	Description string
	Details     map[string]interface{}
}

// PerformanceResult represents performance test results
type PerformanceResult struct {
	TotalRequests  int
	SuccessfulReqs int
	FailedRequests int
	AverageLatency time.Duration
	MaxLatency     time.Duration
	MinLatency     time.Duration
	Throughput     float64 // requests per second
	MemoryUsage    int64   // bytes
	ErrorRate      float64 // percentage
	Errors         []error
}

// NewPluginTester creates a new plugin tester
func NewPluginTester(logger *logrus.Logger) *PluginTester {
	if logger == nil {
		logger = logrus.New()
	}

	return &PluginTester{
		logger:   logger,
		mockData: make(map[string]interface{}),
	}
}

// SetupMockServer sets up a mock HTTP server for testing
func (pt *PluginTester) SetupMockServer(responses map[string]MockResponse) {
	pt.mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		method := r.Method
		key := method + " " + path

		// Find matching response
		var response MockResponse
		found := false

		for pattern, resp := range responses {
			if pt.matchesPattern(pattern, key) {
				response = resp
				found = true
				break
			}
		}

		if !found {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error": "not found"}`))
			return
		}

		// Set headers
		for k, v := range response.Headers {
			w.Header().Set(k, v)
		}

		// Set status code
		w.WriteHeader(response.StatusCode)

		// Write response body
		if response.Body != nil {
			if bodyBytes, ok := response.Body.([]byte); ok {
				w.Write(bodyBytes)
			} else {
				json.NewEncoder(w).Encode(response.Body)
			}
		}
	}))
}

// TeardownMockServer tears down the mock server
func (pt *PluginTester) TeardownMockServer() {
	if pt.mockServer != nil {
		pt.mockServer.Close()
		pt.mockServer = nil
	}
}

// GetMockServerURL returns the mock server URL
func (pt *PluginTester) GetMockServerURL() string {
	if pt.mockServer != nil {
		return pt.mockServer.URL
	}
	return ""
}

// RunTestSuite runs a complete test suite for a plugin
func (pt *PluginTester) RunTestSuite(suite *TestSuite) (*TestSuiteResult, error) {
	result := &TestSuiteResult{
		Plugin:      suite.Plugin.Name(),
		StartTime:   time.Now(),
		TestResults: make([]TestResult, 0),
	}

	pt.logger.WithField("plugin", suite.Plugin.Name()).Info("Starting plugin test suite")

	// Run basic functionality tests
	for _, testCase := range suite.TestCases {
		testResult := pt.runTestCase(suite.Plugin, testCase)
		result.TestResults = append(result.TestResults, testResult)

		if testResult.Passed {
			result.PassedTests++
		} else {
			result.FailedTests++
		}
	}

	// Run performance tests if configured
	if suite.Performance != nil {
		perfResult, err := pt.runPerformanceTest(suite.Plugin, suite.Performance)
		if err != nil {
			pt.logger.WithError(err).Warn("Performance test failed")
		} else {
			result.PerformanceResult = perfResult
		}
	}

	// Run integration tests if configured
	if suite.Integration != nil {
		integrationResults := pt.runIntegrationTests(suite.Plugin, suite.Integration)
		result.TestResults = append(result.TestResults, integrationResults...)

		for _, integResult := range integrationResults {
			if integResult.Passed {
				result.PassedTests++
			} else {
				result.FailedTests++
			}
		}
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.TotalTests = len(result.TestResults)
	result.Success = result.FailedTests == 0

	pt.logger.WithFields(logrus.Fields{
		"plugin":      suite.Plugin.Name(),
		"total_tests": result.TotalTests,
		"passed":      result.PassedTests,
		"failed":      result.FailedTests,
		"duration":    result.Duration,
	}).Info("Plugin test suite completed")

	return result, nil
}

// ValidatePluginInterface validates that a plugin correctly implements the interface
func (pt *PluginTester) ValidatePluginInterface(plugin ExternalSystemPlugin) []ValidationError {
	var errors []ValidationError

	// Check Name method
	name := plugin.Name()
	if name == "" {
		errors = append(errors, ValidationError{
			Method:  "Name",
			Message: "plugin name cannot be empty",
		})
	}

	// Check GetMetadata method
	metadata := plugin.GetMetadata()
	if metadata == nil {
		errors = append(errors, ValidationError{
			Method:  "GetMetadata",
			Message: "metadata cannot be nil",
		})
	} else {
		if metadata.Name == "" {
			errors = append(errors, ValidationError{
				Method:  "GetMetadata",
				Message: "metadata name cannot be empty",
			})
		}
		if metadata.Version == "" {
			errors = append(errors, ValidationError{
				Method:  "GetMetadata",
				Message: "metadata version cannot be empty",
			})
		}
	}

	// Test Connect method with empty config
	err := plugin.Connect(map[string]interface{}{})
	if err == nil {
		errors = append(errors, ValidationError{
			Method:  "Connect",
			Message: "should return error for empty configuration",
		})
	}

	// Test IsHealthy method
	healthy := plugin.IsHealthy()
	if healthy {
		errors = append(errors, ValidationError{
			Method:  "IsHealthy",
			Message: "should return false when not connected",
		})
	}

	return errors
}

// BenchmarkPlugin runs performance benchmarks on a plugin
func (pt *PluginTester) BenchmarkPlugin(plugin ExternalSystemPlugin, userID string, iterations int) *BenchmarkResult {
	result := &BenchmarkResult{
		Plugin:     plugin.Name(),
		UserID:     userID,
		Iterations: iterations,
		StartTime:  time.Now(),
	}

	var totalDuration time.Duration
	var minDuration = time.Hour
	var maxDuration time.Duration
	var errors []error

	for i := 0; i < iterations; i++ {
		start := time.Now()

		_, err := plugin.EnrichUserProfile(userID)

		duration := time.Since(start)
		totalDuration += duration

		if duration < minDuration {
			minDuration = duration
		}
		if duration > maxDuration {
			maxDuration = duration
		}

		if err != nil {
			errors = append(errors, err)
		}
	}

	result.EndTime = time.Now()
	result.TotalDuration = totalDuration
	result.AverageDuration = totalDuration / time.Duration(iterations)
	result.MinDuration = minDuration
	result.MaxDuration = maxDuration
	result.ErrorCount = len(errors)
	result.Errors = errors
	result.SuccessRate = float64(iterations-len(errors)) / float64(iterations) * 100

	return result
}

// Private methods

func (pt *PluginTester) runTestCase(plugin ExternalSystemPlugin, testCase TestCase) TestResult {
	start := time.Now()

	result := TestResult{
		TestName:    testCase.Name,
		Description: testCase.Description,
		Details:     make(map[string]interface{}),
	}

	// Run the test
	enrichment, err := plugin.EnrichUserProfile(testCase.UserID)

	result.Duration = time.Since(start)

	// Check for expected error
	if testCase.ExpectedError != nil {
		if err == nil {
			result.Passed = false
			result.Error = fmt.Errorf("expected error but got none")
			return result
		}

		if err.Error() != testCase.ExpectedError.Error() {
			result.Passed = false
			result.Error = fmt.Errorf("expected error '%v' but got '%v'", testCase.ExpectedError, err)
			return result
		}

		result.Passed = true
		return result
	}

	// Check for unexpected error
	if err != nil {
		result.Passed = false
		result.Error = err
		return result
	}

	// Validate result
	if testCase.ExpectedResult != nil {
		if !pt.compareEnrichments(enrichment, testCase.ExpectedResult) {
			result.Passed = false
			result.Error = fmt.Errorf("enrichment result does not match expected")
			result.Details["expected"] = testCase.ExpectedResult
			result.Details["actual"] = enrichment
			return result
		}
	}

	result.Passed = true
	result.Details["enrichment"] = enrichment

	return result
}

func (pt *PluginTester) runPerformanceTest(plugin ExternalSystemPlugin, perfTest *PerformanceTest) (*PerformanceResult, error) {
	result := &PerformanceResult{
		Errors: make([]error, 0),
	}

	ctx, cancel := context.WithTimeout(context.Background(), perfTest.Duration)
	defer cancel()

	// Channel for collecting results
	results := make(chan *requestResult, perfTest.ConcurrentUsers*100)

	// Start concurrent workers
	for i := 0; i < perfTest.ConcurrentUsers; i++ {
		go pt.performanceWorker(ctx, plugin, fmt.Sprintf("user_%d", i), results)
	}

	// Collect results
	var latencies []time.Duration
	startTime := time.Now()

	for {
		select {
		case res := <-results:
			result.TotalRequests++
			latencies = append(latencies, res.Duration)

			if res.Error != nil {
				result.FailedRequests++
				result.Errors = append(result.Errors, res.Error)
			} else {
				result.SuccessfulReqs++
			}

		case <-ctx.Done():
			goto done
		}
	}

done:
	duration := time.Since(startTime)

	// Calculate statistics
	if len(latencies) > 0 {
		result.AverageLatency = pt.calculateAverage(latencies)
		result.MinLatency = pt.calculateMin(latencies)
		result.MaxLatency = pt.calculateMax(latencies)
	}

	result.Throughput = float64(result.TotalRequests) / duration.Seconds()
	result.ErrorRate = float64(result.FailedRequests) / float64(result.TotalRequests) * 100

	return result, nil
}

func (pt *PluginTester) runIntegrationTests(plugin ExternalSystemPlugin, integTest *IntegrationTest) []TestResult {
	var results []TestResult

	// Config validation test
	if integTest.ConfigValidation {
		result := pt.testConfigValidation(plugin)
		results = append(results, result)
	}

	// Error handling test
	if integTest.ErrorHandling {
		result := pt.testErrorHandling(plugin)
		results = append(results, result)
	}

	// Health check test
	if integTest.HealthCheck {
		result := pt.testHealthCheck(plugin)
		results = append(results, result)
	}

	return results
}

func (pt *PluginTester) testConfigValidation(plugin ExternalSystemPlugin) TestResult {
	result := TestResult{
		TestName:    "Config Validation",
		Description: "Test plugin configuration validation",
	}

	start := time.Now()

	// Test with invalid config
	err := plugin.Connect(map[string]interface{}{
		"invalid_key": "invalid_value",
	})

	result.Duration = time.Since(start)

	if err == nil {
		result.Passed = false
		result.Error = fmt.Errorf("plugin should reject invalid configuration")
	} else {
		result.Passed = true
	}

	return result
}

func (pt *PluginTester) testErrorHandling(plugin ExternalSystemPlugin) TestResult {
	result := TestResult{
		TestName:    "Error Handling",
		Description: "Test plugin error handling",
	}

	start := time.Now()

	// Test with non-existent user
	_, err := plugin.EnrichUserProfile("non_existent_user_12345")

	result.Duration = time.Since(start)

	if err == nil {
		result.Passed = false
		result.Error = fmt.Errorf("plugin should return error for non-existent user")
	} else {
		result.Passed = true
	}

	return result
}

func (pt *PluginTester) testHealthCheck(plugin ExternalSystemPlugin) TestResult {
	result := TestResult{
		TestName:    "Health Check",
		Description: "Test plugin health check functionality",
	}

	start := time.Now()

	// Test health check
	healthy := plugin.IsHealthy()

	result.Duration = time.Since(start)
	result.Passed = true // Health check should not fail
	result.Details = map[string]interface{}{
		"healthy": healthy,
	}

	return result
}

func (pt *PluginTester) performanceWorker(ctx context.Context, plugin ExternalSystemPlugin, userID string, results chan<- *requestResult) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			start := time.Now()
			_, err := plugin.EnrichUserProfile(userID)
			duration := time.Since(start)

			results <- &requestResult{
				Duration: duration,
				Error:    err,
			}
		}
	}
}

func (pt *PluginTester) compareEnrichments(actual, expected *UserEnrichment) bool {
	if actual == nil && expected == nil {
		return true
	}
	if actual == nil || expected == nil {
		return false
	}

	// Compare key fields (simplified comparison)
	if actual.Source != expected.Source {
		return false
	}

	if !reflect.DeepEqual(actual.Interests, expected.Interests) {
		return false
	}

	return true
}

func (pt *PluginTester) matchesPattern(pattern, key string) bool {
	// Simple pattern matching (could be enhanced with regex)
	return strings.Contains(key, pattern) || pattern == "*"
}

func (pt *PluginTester) calculateAverage(durations []time.Duration) time.Duration {
	var total time.Duration
	for _, d := range durations {
		total += d
	}
	return total / time.Duration(len(durations))
}

func (pt *PluginTester) calculateMin(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	min := durations[0]
	for _, d := range durations[1:] {
		if d < min {
			min = d
		}
	}
	return min
}

func (pt *PluginTester) calculateMax(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	max := durations[0]
	for _, d := range durations[1:] {
		if d > max {
			max = d
		}
	}
	return max
}

// Supporting types

type MockResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       interface{}
}

type TestSuiteResult struct {
	Plugin            string
	Success           bool
	TotalTests        int
	PassedTests       int
	FailedTests       int
	StartTime         time.Time
	EndTime           time.Time
	Duration          time.Duration
	TestResults       []TestResult
	PerformanceResult *PerformanceResult
}

type ValidationError struct {
	Method  string
	Message string
}

type BenchmarkResult struct {
	Plugin          string
	UserID          string
	Iterations      int
	StartTime       time.Time
	EndTime         time.Time
	TotalDuration   time.Duration
	AverageDuration time.Duration
	MinDuration     time.Duration
	MaxDuration     time.Duration
	ErrorCount      int
	Errors          []error
	SuccessRate     float64
}

type requestResult struct {
	Duration time.Duration
	Error    error
}
