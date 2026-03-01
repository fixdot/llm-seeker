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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	version = "0.1.0-beta3"

	// Probe timeouts
	timeoutProbeFast    = 800 * time.Millisecond
	timeoutProbeFoundry = 3 * time.Second // HTTP probe timeout for Foundry

	// Foundry CLI timeout (separate from HTTP)
	timeoutFoundryCLI = 10 * time.Second

	// Foundry cache
	foundryCacheTTL      = 60 * time.Second
	foundryCacheFileName = "foundry_url"

	// Environment variables
	envPorts = "LLM_SEEKER_PORTS"
	envOrder = "LLM_SEEKER_ORDER"

	exitFound    = 0
	exitNotFound = 1
	exitInternal = 2
)

var (
	// Default ports commonly used by local LLM servers.
	// NOTE: We do not sort to preserve priority.
	defaultPorts = []int{11434, 1234, 8000, 8080}

	// Default provider order when LLM_SEEKER_ORDER is set (listing mode).
	// (You can override with LLM_SEEKER_ORDER.)
	defaultOrder = []string{"ollama", "lmstudio", "vllm", "generic", "foundry"}

	// Fast path host priority (first match wins).
	hostsFast = []string{
		"127.0.0.1",
	}

	// Listing mode host priority.
	hostsList = []string{
		"127.0.0.1",
		"localhost",
	}

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

	clientFast := &http.Client{Timeout: timeoutProbeFast}
	clientFoundry := &http.Client{Timeout: timeoutProbeFoundry}

	// If ORDER is set, it acts as both "priority" and "filter".
	if ord, ok, specified := loadOrder(); specified {
		if !ok {
			internalError()
		}
		found := make([]string, 0, 4)
		seen := make(map[string]struct{})
		runPlanList(ord, clientFast, clientFoundry, &found, seen)
		printOrNotFound(found)
	}

	// If PORTS is set (and ORDER is not), scan only those ports (listing mode).
	// Foundry is opt-in via LLM_SEEKER_ORDER.
	if ports, ok, specified := loadPorts(); specified {
		if !ok {
			internalError()
		}
		found := make([]string, 0, 4)
		seen := make(map[string]struct{})

		for _, p := range ports {
			probePortAllHosts(clientFast, hostsList, p, &found, seen)
		}
		printOrNotFound(found)
	}

	// Default behavior (no env): return the FIRST detected endpoint and exit.
	// Priority: Ollama -> LM Studio -> vLLM -> generic.
	if u, ok := probeFirstOnPort(clientFast, 11434); ok {
		fmt.Println(u)
		os.Exit(exitFound)
	}
	if u, ok := probeFirstOnPort(clientFast, 1234); ok {
		fmt.Println(u)
		os.Exit(exitFound)
	}
	if u, ok := probeFirstOnPort(clientFast, 8000); ok {
		fmt.Println(u)
		os.Exit(exitFound)
	}
	if u, ok := probeFirstOnPort(clientFast, 8080); ok {
		fmt.Println(u)
		os.Exit(exitFound)
	}

	fmt.Fprintln(os.Stderr, "llm-seeker: no endpoint found")
	os.Exit(exitNotFound)
}

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
LLM_SEEKER_PORTS   override port list (comma-separated)
LLM_SEEKER_ORDER   override scan priority and/or limit providers:
                   ollama, foundry, lmstudio, vllm, generic
                   (comma-separated; acts as filter when set)

Options:
-h, --help
-V, --version
`
	fmt.Print(help)
}

func internalError() {
	fmt.Fprintln(os.Stderr, "llm-seeker: internal error")
	os.Exit(exitInternal)
}

// loadPorts loads port configuration from environment variable.
// Returns (ports, ok, specified).
func loadPorts() ([]int, bool, bool) {
	raw := strings.TrimSpace(os.Getenv(envPorts))
	if raw == "" {
		return nil, true, false
	}

	parts := strings.Split(raw, ",")
	ports := make([]int, 0, len(parts))
	seen := make(map[int]struct{})

	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s == "" {
			continue
		}
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > 65535 {
			return nil, false, true
		}
		if _, exists := seen[n]; exists {
			continue
		}
		seen[n] = struct{}{}
		ports = append(ports, n)
	}

	if len(ports) == 0 {
		return nil, false, true
	}

	return ports, true, true
}

// loadOrder loads provider scan order from envOrder.
// Returns (order, ok, specified).
// When specified, it also acts as a filter (only listed providers are scanned).
func loadOrder() ([]string, bool, bool) {
	raw := strings.TrimSpace(os.Getenv(envOrder))
	if raw == "" {
		return nil, true, false
	}

	parts := strings.Split(raw, ",")
	order := make([]string, 0, len(parts))
	seen := make(map[string]struct{})

	for _, p := range parts {
		s := strings.ToLower(strings.TrimSpace(p))
		if s == "" {
			continue
		}
		if !isKnownProvider(s) {
			// Unknown tokens are treated as invalid to keep behavior predictable.
			return nil, false, true
		}
		if _, exists := seen[s]; exists {
			continue
		}
		seen[s] = struct{}{}
		order = append(order, s)
	}

	if len(order) == 0 {
		return nil, false, true
	}

	return order, true, true
}

func isKnownProvider(s string) bool {
	switch s {
	case "ollama", "foundry", "lmstudio", "vllm", "generic":
		return true
	default:
		return false
	}
}

// runPlanList executes a provider plan (order is already filtered) and LISTS all hits.
func runPlanList(order []string, clientFast, clientFoundry *http.Client, found *[]string, seen map[string]struct{}) {
	for _, p := range order {
		switch p {
		case "ollama":
			probePortAllHosts(clientFast, hostsList, 11434, found, seen)
		case "lmstudio":
			probePortAllHosts(clientFast, hostsList, 1234, found, seen)
		case "vllm":
			probePortAllHosts(clientFast, hostsList, 8000, found, seen)
		case "generic":
			probePortAllHosts(clientFast, hostsList, 8080, found, seen)
		case "foundry":
			probeFoundryList(clientFoundry, found, seen)
		}
	}
}

// probeFirstOnPort probes a fixed port using the fast host list and returns the first match.
func probeFirstOnPort(client *http.Client, port int) (string, bool) {
	for _, host := range hostsFast {
		baseURL, ok := probeHostPort(client, host, port)
		if ok {
			return baseURL, true
		}
	}
	return "", false
}

// probePortAllHosts probes a fixed port using a given host list and appends all matches.
func probePortAllHosts(client *http.Client, hosts []string, port int, found *[]string, seen map[string]struct{}) {
	for _, host := range hosts {
		baseURL, ok := probeHostPort(client, host, port)
		if !ok {
			continue
		}
		if _, exists := seen[baseURL]; exists {
			continue
		}
		seen[baseURL] = struct{}{}
		*found = append(*found, baseURL)
	}
}

// probeHostPort checks all known API paths for a given host and port.
func probeHostPort(client *http.Client, host string, port int) (string, bool) {
	for _, path := range paths {
		u := fmt.Sprintf("http://%s:%d%s", host, port, path)
		if !probeURL(client, u) {
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
func probeURL(client *http.Client, u string) bool {
	req, err := http.NewRequest(http.MethodGet, u, nil)
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

// probeFoundryModels tries GET {base}/v1/models with a tiny retry (HTTP only).
// It does NOT rerun the Foundry CLI.
func probeFoundryModels(client *http.Client, base string) bool {
	for i := 0; i < 2; i++ {
		if probeURL(client, base+"/v1/models") {
			return true
		}
		time.Sleep(150 * time.Millisecond)
	}
	return false
}

// probeFoundryFirst detects Foundry and returns the base "/v1" (single result).
// Foundry is opt-in via LLM_SEEKER_ORDER.
//
// Strategy:
//  1) Try cached base URL (TTL + format validation) and verify by HTTP.
//  2) If cache fails, run `foundry service status` once (with timeout), parse URL, verify by HTTP, then cache.
func probeFoundryFirst(client *http.Client) (string, bool) {
	// 1) Cache first
	if base, ok := readFoundryCache(); ok {
		if probeFoundryModels(client, base) {
			// refresh cache timestamp (best-effort)
			_ = writeFoundryCache(base)
			return base + "/v1", true
		}
		// cache is stale or wrong; continue to CLI
	}

	// 2) CLI once
	base, ok := discoverFoundryBaseURL()
	if !ok {
		return "", false
	}
	if !probeFoundryModels(client, base) {
		return "", false
	}

	_ = writeFoundryCache(base) // best-effort
	return base + "/v1", true
}

// probeFoundryList detects Foundry and appends the base "/v1" (listing mode).
func probeFoundryList(client *http.Client, found *[]string, seen map[string]struct{}) {
	u, ok := probeFoundryFirst(client)
	if !ok {
		return
	}
	if _, exists := seen[u]; exists {
		return
	}
	seen[u] = struct{}{}
	*found = append(*found, u)
}

// stripANSI removes ANSI escape sequences (e.g. color codes) from terminal output.
func stripANSI(s string) string {
	// CSI: ESC [ ... letter
	re := regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)
	return re.ReplaceAllString(s, "")
}

// trimURLTail removes characters that often stick to URLs in CLI output.
func trimURLTail(s string) string {
	return strings.TrimRightFunc(s, func(r rune) bool {
		// Keep common URL characters; drop punctuation/control/emoji leftovers.
		if r <= 32 || r == '"' || r == '\'' {
			return true
		}
		// Common trailing punctuation in prose output
		switch r {
		case '.', ',', ')', ']', '}', '>', ';':
			return true
		}
		// Drop non-ASCII odd tails conservatively
		if r > 127 && unicode.IsPunct(r) {
			return true
		}
		return false
	})
}

// discoverFoundryBaseURL detects a running Foundry Local service and extracts its base URL.
// It runs `foundry service status` once, with a timeout to avoid long hangs when the service is stopped.
func discoverFoundryBaseURL() (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutFoundryCLI)
	defer cancel()

	cmd := exec.CommandContext(ctx, "foundry", "service", "status")

	// Capture both stdout and stderr
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		// timeout/cancel
		return "", false
	}
	if err != nil {
		return "", false
	}

	s := stripANSI(string(out))

	// Extract all URLs and use the last one (more robust if output has multiple URLs).
	re := regexp.MustCompile(`https?://[^\s]+`)
	all := re.FindAllString(s, -1)
	if len(all) == 0 {
		return "", false
	}

	raw := trimURLTail(all[len(all)-1])

	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", false
	}

	// Return scheme + host only (drop path)
	return fmt.Sprintf("%s://%s", u.Scheme, u.Host), true
}

func printOrNotFound(found []string) {
	if len(found) == 0 {
		fmt.Fprintln(os.Stderr, "llm-seeker: no endpoint found")
		os.Exit(exitNotFound)
	}

	for _, u := range found {
		fmt.Println(u)
	}
	os.Exit(exitFound)
}

// ---------- Foundry cache ----------

func foundryCachePath() (string, bool) {
	dir, err := os.UserCacheDir()
	if err != nil || dir == "" {
		return "", false
	}
	// e.g. ~/Library/Caches/llm-seeker/foundry_url
	return filepath.Join(dir, "llm-seeker", foundryCacheFileName), true
}

func validateFoundryBaseURL(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 200 {
		return "", false
	}
	if strings.ContainsRune(raw, '\x1b') {
		return "", false
	}

	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", false
	}

	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		return "", false
	}

	// Keep it local-only for safety (Foundry Local)
	if host != "127.0.0.1" && host != "localhost" {
		return "", false
	}

	p, err := strconv.Atoi(portStr)
	if err != nil || p < 1 || p > 65535 {
		return "", false
	}

	return fmt.Sprintf("%s://%s", u.Scheme, u.Host), true
}

func readFoundryCache() (string, bool) {
	path, ok := foundryCachePath()
	if !ok {
		return "", false
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}

	lines := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	if len(lines) < 2 {
		return "", false
	}

	base, ok := validateFoundryBaseURL(lines[0])
	if !ok {
		return "", false
	}

	tsStr := strings.TrimSpace(lines[1])
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil || ts <= 0 {
		return "", false
	}

	age := time.Since(time.Unix(ts, 0))
	if age < 0 || age > foundryCacheTTL {
		return "", false
	}

	return base, true
}

func writeFoundryCache(base string) error {
	path, ok := foundryCachePath()
	if !ok {
		return fmt.Errorf("cache dir unavailable")
	}

	base, ok = validateFoundryBaseURL(base)
	if !ok {
		return fmt.Errorf("invalid base url")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, "foundry_url_*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	_, werr := tmp.WriteString(base + "\n" + strconv.FormatInt(time.Now().Unix(), 10) + "\n")
	cerr := tmp.Close()
	if werr != nil {
		_ = os.Remove(tmpName)
		return werr
	}
	if cerr != nil {
		_ = os.Remove(tmpName)
		return cerr
	}

	// Atomic replace
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}

	_ = os.Chmod(path, 0o600)
	return nil
}
