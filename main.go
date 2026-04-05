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
	version = "0.2.0"

	// Fast probe timeout for fixed-port providers.
	timeoutProbeFast = 800 * time.Millisecond

	// Foundry probe timeout (models can be slower).
	timeoutProbeFoundry = 3 * time.Second

	// Foundry CLI timeouts.
	timeoutFoundryCLIShort = 3 * time.Second
	timeoutFoundryCLILong  = 10 * time.Second

	// Foundry cache.
	foundryCacheTTL      = 60 * time.Minute
	foundryCacheFileName = "foundry_url"

	// Foundry stability knobs.
	foundryStatusRetry      = 2
	foundryStatusRetrySleep = 300 * time.Millisecond
	foundryWarmupSleep      = 300 * time.Millisecond
	foundryModelsRetry      = 5
	foundryModelsRetrySleep = 150 * time.Millisecond
	foundryWholeRetry       = 3
	foundryWholeRetrySleep  = 250 * time.Millisecond

	foundryWarmupPath    = "/openai/status"
	foundryWarmupTimeout = 1 * time.Second // warmup should be quick

	envPorts = "LLM_SEEKER_PORTS"
	envOrder = "LLM_SEEKER_ORDER"

	exitFound    = 0
	exitNotFound = 1
	exitInternal = 2
)

var (
	defaultPorts = []int{11434, 1234, 8000, 8080}

	defaultOrder = []string{"ollama", "lmstudio", "vllm", "generic", "foundry"}

	hostsList = []string{"127.0.0.1", "localhost"}

	paths = []string{"/v1/models"}

	// Precompiled regex (avoid recompiling repeatedly).
	reANSI = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)
	reURL  = regexp.MustCompile(`https?://[^\s]+`)
)

type labeledEndpoint struct {
	provider string
	url      string
}

func main() {

	if len(os.Args) > 2 {
		internalError()
	}

	// --env: print a single export line for the FIRST detected endpoint.
	flagEnv := false

	if len(os.Args) == 2 {
		switch os.Args[1] {
		case "-h", "--help":
			printHelp()
			os.Exit(exitFound)
		case "-V", "--version":
			fmt.Println(version)
			os.Exit(exitFound)
		case "--env":
			flagEnv = true
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

		// --env mode: pick FIRST match by ORDER priority (not listing).
		if flagEnv {
			if u, ok := runPlanFirst(ord, clientFast, clientFoundry); ok {
				printEnv(u)
				os.Exit(exitFound)
			}
			fmt.Fprintln(os.Stderr, "llm-seeker: no endpoint found")
			os.Exit(exitNotFound)
		}

		// listing mode with provider labels
		found := make([]labeledEndpoint, 0, 4)
		seen := make(map[string]struct{})

		runPlanList(ord, clientFast, clientFoundry, &found, seen)

		printLabeledOrNotFound(found)
	}

	// If PORTS is set (and ORDER is not), scan only those ports (listing mode).
	// Foundry is opt-in via LLM_SEEKER_ORDER.
	if ports, ok, specified := loadPorts(); specified {
		if !ok {
			internalError()
		}

		// --env mode: pick FIRST match by ports order (not listing).
		if flagEnv {
			for _, p := range ports {
				if u, ok := probeFirstOnPort(clientFast, p); ok {
					printEnv(u)
					os.Exit(exitFound)
				}
			}
			fmt.Fprintln(os.Stderr, "llm-seeker: no endpoint found")
			os.Exit(exitNotFound)
		}

		// listing mode (existing behavior)
		found := make([]string, 0, 4)
		seen := make(map[string]struct{})

		for _, p := range ports {
			probePortAllHosts(clientFast, hostsList, p, &found, seen)
		}

		printOrNotFound(found)
	}

	// Default behavior: return the FIRST detected endpoint.
	for _, p := range defaultPorts {
		if u, ok := probeFirstOnPort(clientFast, p); ok {
			if flagEnv {
				printEnv(u)
			} else {
				fmt.Println(u)
			}
			os.Exit(exitFound)
		}
	}

	fmt.Fprintln(os.Stderr, "llm-seeker: no endpoint found")
	os.Exit(exitNotFound)
}

func printHelp() {

	help := `llm-seeker — discover OpenAI-compatible LLM endpoints

Usage:
llm-seeker
llm-seeker --env

Output:
Default: prints the first detected base URL to stdout
--env:   prints one export line (OPENAI_BASE_URL) to stdout
ORDER:   prints matching endpoints as: provider base_url

Exit codes:
0  endpoint found
1  no endpoint found
2  invalid input

Environment:
LLM_SEEKER_PORTS   override port list (comma-separated)
LLM_SEEKER_ORDER   set scan priority and/or limit providers
                   ollama, foundry, lmstudio, vllm, generic
                   comma-separated; also enables listing mode
                   when set, LLM_SEEKER_ORDER takes precedence
                   over LLM_SEEKER_PORTS

Options:
-h, --help
-V, --version
--env
`

	fmt.Print(help)
}

func internalError() {
	fmt.Fprintln(os.Stderr, "llm-seeker: invalid input")
	os.Exit(exitInternal)
}

func printEnv(baseURL string) {
	fmt.Printf("export OPENAI_BASE_URL=%s\n", baseURL)
}

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

// runPlanFirst returns the FIRST detected endpoint by provider priority.
// This is used for --env mode (single endpoint export).
func runPlanFirst(order []string, clientFast, clientFoundry *http.Client) (string, bool) {

	for _, p := range order {

		switch p {

		case "ollama":
			if u, ok := probeFirstOnPort(clientFast, 11434); ok {
				return u, true
			}

		case "lmstudio":
			if u, ok := probeFirstOnPort(clientFast, 1234); ok {
				return u, true
			}

		case "vllm":
			if u, ok := probeFirstOnPort(clientFast, 8000); ok {
				return u, true
			}

		case "generic":
			if u, ok := probeFirstOnPort(clientFast, 8080); ok {
				return u, true
			}

		case "foundry":
			if u, ok := probeFoundryFirst(clientFoundry); ok {
				return u, true
			}
		}
	}

	return "", false
}

func runPlanList(order []string, clientFast, clientFoundry *http.Client, found *[]labeledEndpoint, seen map[string]struct{}) {

	for _, p := range order {

		switch p {

		case "ollama":
			probePortAllHostsLabeled(clientFast, hostsList, 11434, "ollama", found, seen)

		case "lmstudio":
			probePortAllHostsLabeled(clientFast, hostsList, 1234, "lmstudio", found, seen)

		case "vllm":
			probePortAllHostsLabeled(clientFast, hostsList, 8000, "vllm", found, seen)

		case "generic":
			probePortAllHostsLabeled(clientFast, hostsList, 8080, "generic", found, seen)

		case "foundry":
			probeFoundryList(clientFoundry, found, seen)
		}
	}
}

func probeFirstOnPort(client *http.Client, port int) (string, bool) {

	for _, host := range hostsList {

		baseURL, ok := probeHostPort(client, host, port)

		if ok {
			return baseURL, true
		}
	}

	return "", false
}

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

func probePortAllHostsLabeled(client *http.Client, hosts []string, port int, provider string, found *[]labeledEndpoint, seen map[string]struct{}) {

	for _, host := range hosts {

		baseURL, ok := probeHostPort(client, host, port)

		if !ok {
			continue
		}

		if _, exists := seen[baseURL]; exists {
			continue
		}

		seen[baseURL] = struct{}{}
		*found = append(*found, labeledEndpoint{
			provider: provider,
			url:      baseURL,
		})
	}
}

func probeHostPort(client *http.Client, host string, port int) (string, bool) {

	for _, path := range paths {

		u := fmt.Sprintf("http://%s:%d%s", host, port, path)

		if !probeURL(client, u) {
			continue
		}

		return fmt.Sprintf("http://%s:%d/v1", host, port), true
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

	if _, ok := obj["object"]; ok {
		return true
	}

	if _, ok := obj["data"]; ok {
		return true
	}

	return false
}

// probeFoundryModels tries GET {base}/v1/models with retries.
func probeFoundryModels(client *http.Client, base string) bool {

	for i := 0; i < foundryModelsRetry; i++ {

		if probeURL(client, base+"/v1/models") {
			return true
		}

		time.Sleep(foundryModelsRetrySleep)
	}

	return false
}

// probeFoundryWarmup performs a lightweight reachability request.
// The goal is to "wake up" the service path before /v1/models probing.
func probeFoundryWarmup(base string) {

	c := &http.Client{Timeout: foundryWarmupTimeout}

	req, err := http.NewRequest(http.MethodGet, base+foundryWarmupPath, nil)
	if err != nil {
		return
	}

	resp, err := c.Do(req)
	if err != nil {
		return
	}
	_ = resp.Body.Close()
}

// isFoundryProbablyRunning checks for a Foundry Local agent process quickly.
// If pgrep is not available, it returns false without extra delay.
func isFoundryProbablyRunning() bool {

	if _, err := exec.LookPath("pgrep"); err != nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, "pgrep", "-f", "Inference.Service.Agent")

	out, err := cmd.CombinedOutput()

	if ctx.Err() != nil {
		return false
	}

	if err != nil {
		return false
	}

	return strings.TrimSpace(string(out)) != ""
}

// probeFoundryFirst detects Foundry and returns base "/v1".
// Strategy:
//  1) Try cached base URL (TTL + format validation) and verify by HTTP.
//  2) If cache fails, run status with short timeout (retry a bit), warm up, then probe models.
//  3) If still failing and a Foundry agent is likely running, allow one long status attempt.
func probeFoundryFirst(client *http.Client) (string, bool) {

	// 1) Cache first
	if base, ok := readFoundryCache(); ok {

		// Warm up can help even for cached endpoints.
		probeFoundryWarmup(base)

		if probeFoundryModels(client, base) {
			_ = writeFoundryCache(base)
			return base + "/v1", true
		}
		// Cache may be stale; fall through to CLI discovery.
	}

	// 2) Whole Foundry discovery loop (handles warm-up races).
	for attempt := 0; attempt < foundryWholeRetry; attempt++ {

		base, ok, timedOut := discoverFoundryBaseURLWithRetry(timeoutFoundryCLIShort)

		if ok {

			// Warm up + immediately probe models.
			probeFoundryWarmup(base)

			if probeFoundryModels(client, base) {
				_ = writeFoundryCache(base)
				return base + "/v1", true
			}
		} else if timedOut && isFoundryProbablyRunning() {

			// One long attempt if short timed out but the agent exists.
			base2, ok2, _ := discoverFoundryBaseURL(timeoutFoundryCLILong)
			if ok2 {
				probeFoundryWarmup(base2)

				if probeFoundryModels(client, base2) {
					_ = writeFoundryCache(base2)
					return base2 + "/v1", true
				}
			}
		}

		time.Sleep(foundryWholeRetrySleep)
	}

	return "", false
}

// discoverFoundryBaseURLWithRetry runs status multiple times with small sleeps.
// This targets intermittent "no URL" output from the CLI.
func discoverFoundryBaseURLWithRetry(timeout time.Duration) (string, bool, bool) {

	for i := 0; i < foundryStatusRetry; i++ {

		base, ok, timedOut := discoverFoundryBaseURL(timeout)
		if ok || timedOut {
			return base, ok, timedOut
		}

		time.Sleep(foundryStatusRetrySleep)
	}

	return "", false, false
}

// probeFoundryList detects Foundry and appends base "/v1" (listing mode).
func probeFoundryList(client *http.Client, found *[]labeledEndpoint, seen map[string]struct{}) {

	u, ok := probeFoundryFirst(client)

	if !ok {
		return
	}

	if _, exists := seen[u]; exists {
		return
	}

	seen[u] = struct{}{}
	*found = append(*found, labeledEndpoint{
		provider: "foundry",
		url:      u,
	})
}

// stripANSI removes ANSI escape sequences (e.g. color codes) from terminal output.
func stripANSI(s string) string {
	return reANSI.ReplaceAllString(s, "")
}

// trimURLTail removes characters that often stick to URLs in CLI output.
func trimURLTail(s string) string {
	return strings.TrimRightFunc(s, func(r rune) bool {
		if r <= 32 || r == '"' || r == '\'' {
			return true
		}
		switch r {
		case '.', ',', ')', ']', '}', '>', ';':
			return true
		}
		if r > 127 && unicode.IsPunct(r) {
			return true
		}
		return false
	})
}

// discoverFoundryBaseURL runs `foundry service status` and extracts scheme://host.
// Returns (baseURL, ok, timedOut).
func discoverFoundryBaseURL(timeout time.Duration) (string, bool, bool) {

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "foundry", "service", "status")

	out, err := cmd.CombinedOutput()

	if ctx.Err() != nil {
		return "", false, true
	}

	if err != nil {
		return "", false, false
	}

	s := stripANSI(string(out))

	all := reURL.FindAllString(s, -1)

	if len(all) == 0 {
		return "", false, false
	}

	raw := trimURLTail(all[len(all)-1])

	u, err := url.Parse(raw)

	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", false, false
	}

	return fmt.Sprintf("%s://%s", u.Scheme, u.Host), true, false
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

func printLabeledOrNotFound(found []labeledEndpoint) {

	if len(found) == 0 {
		fmt.Fprintln(os.Stderr, "llm-seeker: no endpoint found")
		os.Exit(exitNotFound)
	}

	for _, e := range found {
		fmt.Printf("%s %s\n", e.provider, e.url)
	}

	os.Exit(exitFound)
}

// ---------- Foundry cache ----------

func foundryCachePath() (string, bool) {

	dir, err := os.UserCacheDir()

	if err != nil || dir == "" {
		return "", false
	}

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

	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}

	_ = os.Chmod(path, 0o600)

	return nil
}
