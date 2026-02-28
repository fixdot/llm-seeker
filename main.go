// llm-seeker (MVP)
// Discover OpenAI-compatible LLM endpoints
//
// Author: fix
// Assisted by: ChatGPT
// Contact: https://github.com/fixdot/llm-seeker
// License: MIT License
//
// stdout: base_url(s), newline-separated
// exit: 0 found / 1 not found / 2 error

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	version      = "0.1.0-beta2"
	timeoutProbe = 800 * time.Millisecond
	
	// Environment variable to override default port list
	envPorts = "LLM_SEEKER_PORTS"
	
	exitFound    = 0
	exitNotFound = 1
	exitInternal = 2
)

var (
	// Default ports commonly used by local LLM servers
	defaultPorts = []int{11434, 1234, 8000, 8080}
	
	// Local hosts to probe
	hosts = []string{"localhost", "127.0.0.1"}
	
	// API paths to check for OpenAI compatibility
	paths = []string{"/v1/models", "/models"}
)

func main() {
	if len(os.Args) > 2 {
		internalError()
	}
	
	if len(os.Args) == 2 {
		switch os.Args[1] {
		case "-h", "--help":
			printHelp()
			os.Exit(exitFound)
		case "-V", "--version":
			fmt.Println(version)
			os.Exit(exitFound)
		default:
			internalError()
		}
	}
	
	ports, ok := loadPorts()
	if !ok {
		internalError()
	}
	
	client := &http.Client{Timeout: timeoutProbe}
	
	// First, try to detect Foundry Local (dynamic port)
	if base, ok := discoverFoundryBaseURL(); ok {
		if probeURL(client, base+"/v1/models") {
			fmt.Println(base + "/v1")
			os.Exit(exitFound)
		}
	}
	
	found := make([]string, 0, 4)
	seen := make(map[string]struct{})
	
	// Probe configured ports and hosts
	for _, port := range ports {
		for _, host := range hosts {
			baseURL, okProbe := probeHostPort(client, host, port)
			if !okProbe {
				continue
			}
			if _, exists := seen[baseURL]; exists {
				continue
			}
			seen[baseURL] = struct{}{}
			found = append(found, baseURL)
		}
	}
	
	if len(found) == 0 {
		fmt.Fprintln(os.Stderr, "llm-seeker: no endpoint found")
		os.Exit(exitNotFound)
	}
	
	for _, u := range found {
		fmt.Println(u)
	}
	
	os.Exit(exitFound)
}

// printHelp prints CLI usage information.
func printHelp() {
	help := `llm-seeker — discover OpenAI-compatible LLM endpoints
	
Usage:
llm-seeker

Output:
Prints detected base_url(s) to stdout (newline-separated)

Exit codes:
0  endpoint found
1  no endpoint found
2  internal error

Environment:
LLM_SEEKER_PORTS   override default port list (comma-separated)

Options:
-h, --help
-V, --version
`
	fmt.Print(help)
}

// internalError prints a generic internal error message and exits.
func internalError() {
	fmt.Fprintln(os.Stderr, "llm-seeker: internal error")
	os.Exit(exitInternal)
}

// loadPorts loads port configuration from environment variable or defaults.
func loadPorts() ([]int, bool) {
	raw := strings.TrimSpace(os.Getenv(envPorts))
	
	if raw == "" {
		ports := append([]int(nil), defaultPorts...)
		sort.Ints(ports)
		return ports, true
	}
	
	parts := strings.Split(raw, ",")
	ports := make([]int, 0, len(parts))
	
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s == "" {
			continue
		}
		
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > 65535 {
			return nil, false
		}
		
		ports = append(ports, n)
	}
	
	if len(ports) == 0 {
		return nil, false
	}
	
	sort.Ints(ports)
	return ports, true
}

// probeHostPort checks all known API paths for a given host and port.
func probeHostPort(client *http.Client, host string, port int) (string, bool) {
	for _, path := range paths {
		url := fmt.Sprintf("http://%s:%d%s", host, port, path)
		
		if !probeURL(client, url) {
			continue
		}
		
		switch path {
		case "/v1/models":
			return fmt.Sprintf("http://%s:%d/v1", host, port), true
		case "/models":
			return fmt.Sprintf("http://%s:%d", host, port), true
		}
	}
	
	return "", false
}

// probeURL verifies that the given URL returns a valid OpenAI-compatible JSON response.
func probeURL(client *http.Client, url string) bool {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return false
	}
	
	const maxBody = 1 << 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return false
	}
	
	body = bytes.TrimSpace(body)
	
	if len(body) == 0 {
		return false
	}
	
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return false
	}
	
	obj, ok := v.(map[string]any)
	if !ok {
		return false
	}
	
	// Check typical OpenAI response fields
	if _, ok := obj["object"]; ok {
		return true
	}
	if _, ok := obj["data"]; ok {
		return true
	}
	
	return false
}

// discoverFoundryBaseURL detects a running Foundry Local service
// and extracts its base URL (dynamic port).
func discoverFoundryBaseURL() (string, bool) {
	cmd := exec.Command("foundry", "service", "status")
	
	// Capture both stdout and stderr
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", false
	}
	
	s := string(out)
	
	// Extract URL like: http://127.0.0.1:57412/openai/status
	re := regexp.MustCompile(`https?://[^\s]+`)
	raw := re.FindString(s)
	if raw == "" {
		return "", false
	}
	
	// Trim trailing punctuation if present
	raw = strings.TrimRight(raw, ".,)")
	
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", false
	}
	
	// Return scheme + host only (drop path)
	base := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	
	return base, true
}
