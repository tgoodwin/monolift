package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	simpleRand "math/rand/v2"
	"mime/multipart"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gonum.org/v1/gonum/stat"
)

// Result holds the outcome of a single HTTP request.
type Result struct {
	Latency time.Duration
	Err     error
}

// Stats aggregates the results of a test step.
type Stats struct {
	TargetRPS    int
	ActualRPS    float64
	SuccessCount int64
	FailureCount int64
	AvgLatency   time.Duration
	P95Latency   time.Duration
}

func main() {
	// --- Command-Line Flags ---
	ip := flag.String("ip", "127.0.0.1", "IP address of the target server.")
	port := flag.Int("port", 8080, "IP port of the target server.")
	stepDuration := flag.Duration("step-duration", 30*time.Second, "Duration to apply load at each RPS level.")
	coolOff := flag.Duration("cool-off", 10*time.Second, "Cool-off period in seconds between load steps.")
	outputFile := flag.String("output-file", "", "Optional path to the output CSV file.")
	numUsers := flag.Int("num-users", 962, "Total number of users in the social graph (for generating random data).")
	rpsLevelStr := flag.String("rps-levels", "", "Custom RPS levels to test, comma-separated (e.g., '100,200,300'). If empty, uses a default sequence.")
	earlyExit := flag.Bool("early-exit", false, "Exit early if no successful requests are made at a given RPS level.")
	// Use all available CPU cores for workers by default.
	concurrency := flag.Int("concurrency", runtime.NumCPU(), "Number of concurrent workers to generate load.")
	workload := flag.String("workload", "save", "Type of workload to run (save, mixed).")
	flag.Parse()

	// --- Test Setup ---
	url := fmt.Sprintf("http://%s:%d", *ip, *port)
	runtime.GOMAXPROCS(*concurrency)

	// A handcrafted sequence of RPS levels designed to produce a detailed curve.
	defaultRPSLevels := []int{
		20, 40, 60, 80, 100, 200, 300, 400, 500, 600, 800, 1000, 1200, 1400, 1600, 1800, 2000, 2200, 2400,
		2600, 2800, 3000, 3400, 3800, 4000, 4400, 4800, 5000,
	}

	var rpsLevels []int
	if *rpsLevelStr != "" {
		levels := strings.Split(*rpsLevelStr, ",")
		for _, level := range levels {
			rps, err := strconv.Atoi(strings.TrimSpace(level))
			if err != nil {
				log.Fatalf("Invalid RPS level '%s' in --rps-levels: %v", level, err)
			}
			rpsLevels = append(rpsLevels, rps)
		}
	} else {
		rpsLevels = defaultRPSLevels
	}

	fmt.Println("--- Go Throughput-Latency Analyzer ---")
	fmt.Printf("Target URL: %s\n", url)
	fmt.Printf("Concurrency: %d workers on %d CPUs\n", *concurrency, runtime.NumCPU())
	fmt.Printf("RPS levels: %v\n", rpsLevels)
	fmt.Printf("Duration per level: %v\n", *stepDuration)
	fmt.Printf("Cool-off period: %v\n", *coolOff)

	// --- CSV Writer Setup ---
	var csvWriter *csv.Writer
	if *outputFile != "" {
		file, err := os.Create(*outputFile)
		if err != nil {
			log.Fatalf("Failed to create output file: %v", err)
		}
		defer file.Close()
		csvWriter = csv.NewWriter(file)
		defer csvWriter.Flush()
		header := []string{"Target RPS", "Actual RPS", "Avg Latency (ms)", "p95 Latency (ms)", "Success Count", "Failure Count"}
		csvWriter.Write(header)
		fmt.Printf("Output CSV file: %s\n", *outputFile)
	}

	//Perform Warmup as the first step in rpsLevels for 60 seconds
	warmup(url) // Perform a warmup request to prime the server
	fmt.Println("Performing long step warmup...")
	runTestStep(url, rpsLevels[0], *stepDuration*4, *numUsers, *workload)

	fmt.Println(
		"--------------------------------------------------------------------------------",
	)
	fmt.Println(
		"Target RPS | Actual RPS | Avg Latency (ms) | p95 Latency (ms) | Success | Fail",
	)
	fmt.Println(
		"-----------+------------+------------------+------------------+---------+------",
	)

	// --- Main Test Loop ---
	for i, rps := range rpsLevels {
		stats := runTestStep(url, rps, *stepDuration, *numUsers, *workload)

		// Print to console
		fmt.Printf(
			"%10d | %10.1f | %16.1f | %16.1f | %7d | %4d\n",
			stats.TargetRPS,
			stats.ActualRPS,
			float64(stats.AvgLatency.Microseconds())/1000.0,
			float64(stats.P95Latency.Microseconds())/1000.0,
			stats.SuccessCount,
			stats.FailureCount,
		)

		// Write to CSV
		if csvWriter != nil {
			row := []string{
				strconv.Itoa(stats.TargetRPS),
				fmt.Sprintf("%.1f", stats.ActualRPS),
				fmt.Sprintf("%.1f", float64(stats.AvgLatency.Microseconds())/1000.0),
				fmt.Sprintf("%.1f", float64(stats.P95Latency.Microseconds())/1000.0),
				strconv.FormatInt(stats.SuccessCount, 10),
				strconv.FormatInt(stats.FailureCount, 10),
			}
			csvWriter.Write(row)
			csvWriter.Flush()
		}
		if *earlyExit {
			if stats.SuccessCount == 0 && stats.FailureCount > 0 {
				fmt.Printf("Warning: No successful requests at RPS %d.", stats.TargetRPS)
				break // Stop if no successful requests
			}
		}
		// Cool-off period
		if *coolOff > 0 && i < len(rpsLevels)-1 {
			fmt.Printf("Cooling off for %v...\n", *coolOff)
			time.Sleep(*coolOff)
		}
	}
}

func warmup(url string) {
	// Perform a warmup request to prime the server.
	client := &http.Client{Timeout: 30 * time.Second} // Use a longer timeout for warmup

	// Generate a dummy payload for the warmup POST request
	for i := 0; i < 5; i++ {
		fmt.Println("Warming up the server with a POST request...")
		body, contentType, err := generatePostData(1) // numUsers doesn't matter much for warmup
		if err != nil {
			log.Fatalf("Warmup: failed to generate post data: %v", err)
		}

		req, err := http.NewRequest("POST", url+"/save", body)
		if err != nil {
			log.Fatalf("Warmup: failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", contentType)
		resp, err := client.Do(req)
		if err != nil {
			log.Fatalf("Warmup: request error: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			log.Fatalf("Warmup request returned bad status: %d", resp.StatusCode)
		}
	}
	fmt.Println("Warmup complete.")
}

// runTestStep executes the load test for a single RPS level and returns aggregated stats.
func runTestStep(url string, rps int, duration time.Duration, numUsers int, workloadType string) Stats {
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	results := make(chan Result, rps*int(duration.Seconds()))
	var wg sync.WaitGroup

	// Create a reusable HTTP client
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	ticker := time.NewTicker(time.Second / time.Duration(rps))
	defer ticker.Stop()

	// Main request-sending loop
	for {
		select {
		case <-ctx.Done():
			// Test duration is over
			wg.Wait()      // Wait for all in-flight requests to finish
			close(results) // Close the channel to signal the results processor
			return processResults(results, rps, duration)
		case <-ticker.C:
			wg.Add(1)
			go sendRequest(client, url, numUsers, results, &wg, workloadType)
		}
	}
}

func sendSaveRequest(client *http.Client, url string, numUsers int, results chan<- Result, wg *sync.WaitGroup) {
	defer wg.Done()

	body, contentType, err := generatePostData(numUsers)
	if err != nil {
		results <- Result{Err: fmt.Errorf("failed to generate data: %w", err)}
		return
	}

	//modify endpoint to match the save endpoint
	finalURL := url + "/save" // Remove trailing /save if present
	req, err := http.NewRequest("POST", finalURL, body)
	if err != nil {
		results <- Result{Err: fmt.Errorf("failed to create request: %w", err)}
		return
	}
	req.Header.Set("Content-Type", contentType)

	startTime := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(startTime)

	if err != nil {
		results <- Result{Err: err}
		return
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) // Consume the body to allow connection reuse

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		results <- Result{Err: fmt.Errorf("bad status code: %d", resp.StatusCode)}
		return
	}
	results <- Result{Latency: latency, Err: nil}
}

func sendReadRequest(client *http.Client, url string, numUsers int, results chan<- Result, wg *sync.WaitGroup, readUser bool) {
	defer wg.Done()

	// Generate a random user ID for the GET request
	userID := strconv.Itoa(time.Now().Nanosecond() % numUsers)
	userTI := strconv.FormatBool(readUser)
	numPosts := strconv.Itoa((time.Now().Nanosecond() % 10) + 1) // Random number of posts between 1 and 10

	// , fmt.Sprintf("%s?user_id=%s", url, userID) ?
	finalUrl := url + "/timeline" + "?user_id=" + userID + "&user_ti=" + userTI + "&posts=" + numPosts //

	req, err := http.NewRequest("GET", finalUrl, nil)
	if err != nil {
		results <- Result{Err: fmt.Errorf("failed to create request: %w", err)}
		return
	}

	startTime := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(startTime)

	if err != nil {
		results <- Result{Err: err}
		return
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) // Consume the body to allow connection reuse

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		results <- Result{Err: fmt.Errorf("bad status code: %d", resp.StatusCode)}
		return
	}
	results <- Result{Latency: latency, Err: nil}
}

func randRange(min, max int) int {
	return simpleRand.IntN(max-min) + min
}

// sendRequest generates data, sends a single HTTP POST request, and records the result.
func sendRequest(client *http.Client, url string, numUsers int, results chan<- Result, wg *sync.WaitGroup, workloadType string) {

	if workloadType == "save" {
		sendSaveRequest(client, url, numUsers, results, wg)
	} else if workloadType == "mixed" {
		coin_toss := randRange(0, 100)
		if coin_toss < 60 {
			// Read user home data: send a GET request
			sendReadRequest(client, url, numUsers, results, wg, false)
		} else if 60 <= coin_toss && coin_toss < 90 {
			// Save user  data: send a GET request
			sendReadRequest(client, url, numUsers, results, wg, true)
		} else if 90 <= coin_toss && coin_toss < 100 {
			// Mixed workload: send a save request
			sendSaveRequest(client, url, numUsers, results, wg)
		} else {
			sendSaveRequest(client, url, numUsers, results, wg)
		}

	}
}

// processResults collects all results from the channel and calculates final statistics.
func processResults(results <-chan Result, rps int, duration time.Duration) Stats {
	var latencies []time.Duration
	var successCount, failureCount int64

	for res := range results {
		if res.Err != nil {
			failureCount++
		} else {
			successCount++
			latencies = append(latencies, res.Latency)
		}
	}

	stats := Stats{
		TargetRPS:    rps,
		SuccessCount: successCount,
		FailureCount: failureCount,
		ActualRPS:    float64(successCount) / duration.Seconds(),
	}

	if len(latencies) > 0 {
		// Convert latencies to a slice of float64 for gonum.
		floatLatencies := make([]float64, len(latencies))
		for i, l := range latencies {
			floatLatencies[i] = float64(l.Microseconds())
		}

		// Calculate p95 using gonum.
		// Note: gonum's Quantile function expects the data to be sorted.
		sort.Float64s(floatLatencies)
		p95Microseconds := stat.Quantile(0.95, stat.Empirical, floatLatencies, nil)
		stats.P95Latency = time.Duration(p95Microseconds) * time.Microsecond

		// Calculate Average
		var totalLatency time.Duration
		for _, l := range latencies {
			totalLatency += l
		}
		stats.AvgLatency = totalLatency / time.Duration(len(latencies))
	}

	return stats
}

// generatePostData creates the multipart/form-data payload.
func generatePostData(numUsers int) (*bytes.Buffer, string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// 1. User ID
	userID := strconv.Itoa(time.Now().Nanosecond() % numUsers)
	writer.WriteField("user_id", userID)

	// 2. Text content
	text := make([]byte, 256)
	rand.Read(text)
	writer.WriteField("text", fmt.Sprintf("%x", text)) // Simple hex encoding for random bytes

	// 3. Dummy image files
	numImages := (time.Now().Nanosecond() % 4) + 1
	for i := 0; i < numImages; i++ {
		part, _ := writer.CreateFormFile("images", fmt.Sprintf("image_%d.jpg", i))
		imgData := make([]byte, 1024)
		rand.Read(imgData)
		part.Write(imgData)
	}

	err := writer.Close()
	if err != nil {
		return nil, "", err
	}

	return body, writer.FormDataContentType(), nil
}
