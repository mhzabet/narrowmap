package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"narrowmap/internal/extract"
	"narrowmap/internal/model"
)

const version = "0.2.6"

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	return RunWithInput(ctx, args, os.Stdin, stdout, stderr)
}

func RunWithInput(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	cfg, err := parseConfig(args, stderr)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}
	if cfg.version {
		fmt.Fprintf(stdout, "narrowmap %s\n", version)
		return nil
	}

	params := newCollector(stdout, cfg.silent)
	log := newProgress(stderr, !cfg.silent)
	warnings := 0
	warn := func(source string, err error) {
		warnings++
		log.Warn("%s: %v", source, err)
	}

	log.Stage("visible parameter discovery")

	switch {
	case cfg.inputFile != "":
		source := cfg.inputFile
		if source == "-" {
			source = "stdin"
		}
		log.Stage("reading file content: %s", source)
		document, truncated, err := loadSingleFile(cfg.inputFile, cfg.maxBody, stdin)
		if err != nil {
			return err
		}
		if truncated {
			warn(cfg.inputFile, fmt.Errorf("content truncated at %d bytes", cfg.maxBody))
		}
		analyzeDocument(document, cfg, params, warn)

	case cfg.inputDir != "":
		files, err := listFolderFiles(cfg.inputDir)
		if err != nil {
			return err
		}
		if len(files) == 0 {
			return fmt.Errorf("no supported HTML, JavaScript, or JSON files found in %s", cfg.inputDir)
		}
		log.Stage("analyzing %d local files", len(files))
		for _, path := range files {
			if err := ctx.Err(); err != nil {
				return err
			}
			document, truncated, err := loadSingleFile(path, cfg.maxBody, stdin)
			if err != nil {
				warn(path, err)
				continue
			}
			if truncated {
				warn(path, fmt.Errorf("content truncated at %d bytes", cfg.maxBody))
			}
			analyzeDocument(document, cfg, params, warn)
		}

	case cfg.inputLinks != "" || cfg.inputURL != "":
		var links []string
		var err error
		if cfg.inputURL != "" {
			links, err = loadURLInput(cfg.inputURL, stdin)
		} else {
			links, err = loadLinks(cfg.inputLinks, stdin)
		}
		if err != nil {
			return err
		}
		headers, err := parseHeaders(cfg.headers)
		if err != nil {
			return err
		}
		extractCookieNames(headers, params.Add)
		for _, link := range links {
			extract.URLParameters(link, "", params.Add)
		}

		log.Stage(
			"fetching %d URLs (concurrency=%d delay=%s timeout=%s)",
			len(links),
			cfg.concurrent,
			cfg.delay,
			cfg.timeout,
		)
		processed, scripts := fetchAndAnalyze(ctx, links, headers, cfg, params, warn)
		log.Stage("analyzed %d/%d fetched responses", processed, len(links))
		if len(scripts) > 0 && cfg.includeSameOriginJS {
			log.Stage("fetching %d same-origin JavaScript assets referenced by HTML", len(scripts))
			scriptProcessed, _ := fetchAndAnalyze(ctx, scripts, headers, cfg, params, warn)
			log.Stage("analyzed %d/%d JavaScript assets", scriptProcessed, len(scripts))
		} else if len(scripts) > 0 {
			log.Stage("skipped %d same-origin JavaScript assets", len(scripts))
		}
	}

	values := params.Sorted()
	if !cfg.silent {
		for _, value := range values {
			fmt.Fprintln(stdout, value)
		}
	}
	if cfg.output != "" {
		if err := writeLines(cfg.output, values); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		log.Stage("wrote %d parameters to %s", len(values), cfg.output)
	}
	log.Stage("complete: %d unique parameters, %d warnings", len(values), warnings)
	return nil
}

func fetchAndAnalyze(
	ctx context.Context,
	links []string,
	headers http.Header,
	cfg config,
	params *collector,
	warn func(string, error),
) (int, []string) {
	processed := 0
	seenScripts := make(map[string]struct{})
	var scripts []string
	for result := range fetchLinks(ctx, links, headers, cfg.concurrent, cfg.delay, cfg.timeout, cfg.maxBody) {
		if result.err != nil {
			warn("fetch", result.err)
			continue
		}
		processed++
		if result.status >= http.StatusBadRequest {
			warn(result.document.Name, fmt.Errorf("HTTP status %d; response was still analyzed", result.status))
		}
		if result.truncated {
			warn(result.document.Name, fmt.Errorf("response truncated at %d bytes", cfg.maxBody))
		}
		analyzeDocument(result.document, cfg, params, warn)
		if result.document.Kind == model.KindHTML {
			for _, script := range extract.HTMLScriptURLs(result.document.Body, result.document.BaseURL) {
				if _, exists := seenScripts[script]; exists {
					continue
				}
				seenScripts[script] = struct{}{}
				scripts = append(scripts, script)
			}
		}
	}
	return processed, scripts
}

func analyzeDocument(document model.Document, cfg config, params *collector, warn func(string, error)) {
	sourceWarn := func(err error) {
		warn(document.Name, err)
	}
	options := extract.Options{IncludeLowSignal: cfg.allParams}
	if err := extract.Document(document, options, params.Add, sourceWarn); err != nil {
		warn(document.Name, err)
	}
}

func extractCookieNames(headers http.Header, add func(string)) {
	for _, cookieHeader := range headers.Values("Cookie") {
		for _, pair := range strings.Split(cookieHeader, ";") {
			name, _, ok := strings.Cut(strings.TrimSpace(pair), "=")
			if ok {
				add(name)
			}
		}
	}
}
