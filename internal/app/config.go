package app

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"
)

const usageText = `narrowmap discovers parameters, archived routes, and target-specific fuzzing wordlists.

Usage:
  narrowmap --input-links links.txt -v-param [options]
  narrowmap --input-url https://target.example -v-param [options]
  narrowmap --input-folder pages_folder -v-param [options]
  narrowmap --input-file page.html -v-param [options]
  narrowmap --paramgen params.txt [options]
  narrowmap --robofinder target.example [options]
  narrowmap --ojs target.example [options]
  cat links.txt | narrowmap --input-links -v-param [options]
  echo target.example | narrowmap --input-url -v-param [options]
  cat page.html | narrowmap --input-file -v-param [options]
  cat params.txt | narrowmap --paramgen [options]
  cat targets.txt | narrowmap --robofinder [options]
  cat targets.txt | narrowmap --ojs [options]

Input:
  --input-links FILE     File containing URLs; omit FILE or use - for stdin
  -u, --input-url URL    Analyze one URL; omit URL or use - for URLs from stdin
  --input-folder DIR     Recursively analyze local .html, .htm, .js, .mjs, .cjs, and .json files
  --input-file FILE      Analyze one file; omit FILE or use - for content from stdin

Discovery:
  -v-param               Discover visible parameter candidates (default mode)
  --all-params           Include low-signal JavaScript variable names
  --include-same-origin-js
                         Fetch same-origin JavaScript referenced by HTML (default)
  --no-same-origin-js    Do not fetch same-origin JavaScript referenced by HTML

Paramgen:
  --paramgen FILE        Generate a smart wordlist from observed parameters; omit FILE or use - for stdin
  --paramgen-prefixes FILE
                         Add target-specific prefixes from a file
  --paramgen-suffixes FILE
                         Add target-specific suffixes from a file
  --paramgen-limit N     Maximum generated values per input parameter (default 64)

Archive discovery:
  --robofinder TARGET    Extract old endpoints from archived robots.txt versions; omit TARGET for stdin
  --ojs TARGET           Extract endpoints from archived JavaScript contexts; omit TARGET for stdin
  --archive-delay DURATION
                         Serial delay before every archive action (default 2s; minimum 500ms)
  --archive-timeout DURATION
                         Timeout for each waybackurls or replay request (default 90s)
  --archive-retries N    Retries for transient archive failures (default 3)
  --archive-max-versions N
                         Evenly sample at most N distinct versions per file; 0 keeps all (default 0)
  --archive-no-subs      Ask waybackurls not to include subdomains
  --waybackurls-bin FILE Path or command name for tomnomnom/waybackurls (default waybackurls)

HTTP:
  -H 'Name: value'       Add a request header; repeat for multiple headers
  -c, -t, --concurrency N
                         Concurrent HTTP requests (default 3)
  --threads N            Alias for --concurrency
  --delay DURATION       Minimum delay between request starts (default 250ms)
  --timeout DURATION     Per-request timeout (default 20s)
  --max-body SIZE        Maximum response/file size, such as 10MB (default 10MB)

Output:
  -s, --silent           Stream unique findings without progress; paramgen remains sorted
  -o, --output FILE      Also write final sorted findings to a file
  --version              Show the narrowmap version
  -h, --help             Show this help
`

type config struct {
	inputLinks          string
	inputURL            string
	inputDir            string
	inputFile           string
	paramgen            string
	paramgenPrefixes    string
	paramgenSuffixes    string
	paramgenLimit       int
	robofinder          string
	ojs                 string
	archiveDelay        time.Duration
	archiveTimeout      time.Duration
	archiveRetries      int
	archiveMaxVersions  int
	archiveNoSubs       bool
	waybackBin          string
	visible             bool
	allParams           bool
	includeSameOriginJS bool
	silent              bool
	output              string
	headers             headerFlags
	concurrent          int
	delay               time.Duration
	timeout             time.Duration
	maxBody             int64
	version             bool
}

type headerFlags []string

func (h *headerFlags) String() string {
	return strings.Join(*h, ", ")
}

func (h *headerFlags) Set(value string) error {
	if !strings.Contains(value, ":") {
		return fmt.Errorf("header must use 'Name: value' format")
	}
	*h = append(*h, value)
	return nil
}

func parseConfig(args []string, stderr io.Writer) (config, error) {
	var cfg config
	var maxBody string
	args = normalizeStdinArgs(args)

	fs := flag.NewFlagSet("narrowmap", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprint(stderr, usageText)
	}

	fs.StringVar(&cfg.inputLinks, "input-links", "", "")
	fs.StringVar(&cfg.inputURL, "input-url", "", "")
	fs.StringVar(&cfg.inputURL, "u", "", "")
	fs.StringVar(&cfg.inputDir, "input-folder", "", "")
	fs.StringVar(&cfg.inputFile, "input-file", "", "")
	fs.StringVar(&cfg.paramgen, "paramgen", "", "")
	fs.StringVar(&cfg.paramgenPrefixes, "paramgen-prefixes", "", "")
	fs.StringVar(&cfg.paramgenSuffixes, "paramgen-suffixes", "", "")
	fs.IntVar(&cfg.paramgenLimit, "paramgen-limit", 64, "")
	fs.StringVar(&cfg.robofinder, "robofinder", "", "")
	fs.StringVar(&cfg.ojs, "ojs", "", "")
	fs.StringVar(&cfg.ojs, "oJs", "", "")
	fs.DurationVar(&cfg.archiveDelay, "archive-delay", 2*time.Second, "")
	fs.DurationVar(&cfg.archiveTimeout, "archive-timeout", 90*time.Second, "")
	fs.IntVar(&cfg.archiveRetries, "archive-retries", 3, "")
	fs.IntVar(&cfg.archiveMaxVersions, "archive-max-versions", 0, "")
	fs.BoolVar(&cfg.archiveNoSubs, "archive-no-subs", false, "")
	fs.StringVar(&cfg.waybackBin, "waybackurls-bin", "waybackurls", "")
	fs.BoolVar(&cfg.visible, "v-param", false, "")
	fs.BoolVar(&cfg.allParams, "all-params", false, "")
	fs.BoolVar(&cfg.includeSameOriginJS, "include-same-origin-js", true, "")
	fs.BoolVar(&cfg.includeSameOriginJS, "same-origin-js", true, "")
	var noSameOriginJS bool
	fs.BoolVar(&noSameOriginJS, "no-same-origin-js", false, "")
	fs.BoolVar(&cfg.silent, "silent", false, "")
	fs.BoolVar(&cfg.silent, "s", false, "")
	fs.StringVar(&cfg.output, "output", "", "")
	fs.StringVar(&cfg.output, "o", "", "")
	fs.Var(&cfg.headers, "H", "")
	fs.BoolVar(&cfg.version, "version", false, "")
	fs.IntVar(&cfg.concurrent, "concurrency", 3, "")
	fs.IntVar(&cfg.concurrent, "c", 3, "")
	fs.IntVar(&cfg.concurrent, "t", 3, "")
	fs.IntVar(&cfg.concurrent, "threads", 3, "")
	fs.DurationVar(&cfg.delay, "delay", 250*time.Millisecond, "")
	fs.DurationVar(&cfg.timeout, "timeout", 20*time.Second, "")
	fs.StringVar(&maxBody, "max-body", "10MB", "")

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	if fs.NArg() != 0 {
		return cfg, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}
	if cfg.version {
		return cfg, nil
	}
	if noSameOriginJS {
		cfg.includeSameOriginJS = false
	}

	inputs := 0
	for _, value := range []string{cfg.inputLinks, cfg.inputURL, cfg.inputDir, cfg.inputFile, cfg.paramgen, cfg.robofinder, cfg.ojs} {
		if value != "" {
			inputs++
		}
	}
	if inputs != 1 {
		return cfg, errors.New("choose exactly one of --input-links, --input-url, --input-folder, --input-file, --paramgen, --robofinder, or --ojs")
	}
	if cfg.paramgen == "" && (cfg.paramgenPrefixes != "" || cfg.paramgenSuffixes != "") {
		return cfg, errors.New("--paramgen-prefixes and --paramgen-suffixes require --paramgen")
	}
	if cfg.paramgen != "" && (cfg.paramgenPrefixes == "-" || cfg.paramgenSuffixes == "-") {
		return cfg, errors.New("custom paramgen prefix and suffix lists must be files, not stdin")
	}
	if cfg.paramgenLimit < 1 || cfg.paramgenLimit > 1000 {
		return cfg, errors.New("--paramgen-limit must be between 1 and 1000")
	}
	archiveMode := cfg.robofinder != "" || cfg.ojs != ""
	archiveOptionsSet := false
	fs.Visit(func(option *flag.Flag) {
		switch option.Name {
		case "archive-delay", "archive-timeout", "archive-retries", "archive-max-versions", "archive-no-subs", "waybackurls-bin":
			archiveOptionsSet = true
		}
	})
	if archiveOptionsSet && !archiveMode {
		return cfg, errors.New("archive options require --robofinder or --ojs")
	}
	if archiveMode && len(cfg.headers) > 0 {
		return cfg, errors.New("-H is not accepted in archive modes because target credentials must not be sent to archive.org")
	}
	if archiveMode && cfg.archiveDelay < 500*time.Millisecond {
		return cfg, errors.New("--archive-delay must be at least 500ms")
	}
	if cfg.archiveTimeout <= 0 {
		return cfg, errors.New("--archive-timeout must be greater than zero")
	}
	if cfg.archiveRetries < 0 || cfg.archiveRetries > 10 {
		return cfg, errors.New("--archive-retries must be between 0 and 10")
	}
	if cfg.archiveMaxVersions < 0 || cfg.archiveMaxVersions > 10000 {
		return cfg, errors.New("--archive-max-versions must be between 0 and 10000")
	}
	if strings.TrimSpace(cfg.waybackBin) == "" {
		return cfg, errors.New("--waybackurls-bin cannot be empty")
	}
	cfg.visible = true
	if cfg.concurrent < 1 || cfg.concurrent > 100 {
		return cfg, errors.New("--concurrency must be between 1 and 100")
	}
	if cfg.delay < 0 {
		return cfg, errors.New("--delay cannot be negative")
	}
	if cfg.timeout <= 0 {
		return cfg, errors.New("--timeout must be greater than zero")
	}

	size, err := parseByteSize(maxBody)
	if err != nil {
		return cfg, fmt.Errorf("invalid --max-body: %w", err)
	}
	cfg.maxBody = size

	return cfg, nil
}

func normalizeStdinArgs(args []string) []string {
	stdinFlags := map[string]struct{}{
		"--input-file":  {},
		"--input-links": {},
		"--input-url":   {},
		"--paramgen":    {},
		"--robofinder":  {},
		"--ojs":         {},
		"--oJs":         {},
		"-u":            {},
	}

	normalized := make([]string, 0, len(args)+1)
	for index, arg := range args {
		normalized = append(normalized, arg)
		if _, ok := stdinFlags[arg]; !ok {
			continue
		}
		if index+1 == len(args) || args[index+1] != "-" && strings.HasPrefix(args[index+1], "-") {
			normalized = append(normalized, "-")
		}
	}
	return normalized
}
