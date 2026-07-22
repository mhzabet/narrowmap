package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"narrowmap/internal/paramgen"
)

func runParamgen(
	ctx context.Context,
	cfg config,
	stdin io.Reader,
	params *collector,
	log *progress,
	warn func(string, error),
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	source := cfg.paramgen
	if source == "-" {
		source = "stdin"
	}
	log.Stage("reading observed parameters: %s", source)
	inputs, truncated, err := loadWordlist(cfg.paramgen, cfg.maxBody, stdin)
	if err != nil {
		return err
	}
	if truncated {
		warn(source, fmt.Errorf("input truncated at %d bytes", cfg.maxBody))
	}

	prefixes, err := loadOptionalWordlist(cfg.paramgenPrefixes, cfg.maxBody, "prefix", warn)
	if err != nil {
		return err
	}
	suffixes, err := loadOptionalWordlist(cfg.paramgenSuffixes, cfg.maxBody, "suffix", warn)
	if err != nil {
		return err
	}

	result := paramgen.Generate(inputs, paramgen.Options{
		Prefixes:     prefixes,
		Suffixes:     suffixes,
		PerSeedLimit: cfg.paramgenLimit,
	})
	if result.Accepted == 0 {
		return fmt.Errorf("%s contains no valid parameter names", source)
	}

	log.Stage(
		"accepted %d seed parameters (%d invalid, %d duplicate)",
		result.Accepted,
		result.Rejected,
		result.Duplicates,
	)
	for _, value := range result.Values {
		params.Add(value)
	}
	return nil
}

func loadOptionalWordlist(
	path string,
	maxBody int64,
	kind string,
	warn func(string, error),
) ([]string, error) {
	if path == "" {
		return nil, nil
	}
	values, truncated, err := loadWordlist(path, maxBody, nil)
	if err != nil {
		return nil, fmt.Errorf("read paramgen %s list: %w", kind, err)
	}
	if truncated {
		warn(path, fmt.Errorf("%s list truncated at %d bytes", kind, maxBody))
	}
	return values, nil
}

func loadWordlist(path string, maxBody int64, stdin io.Reader) ([]string, bool, error) {
	var (
		body      []byte
		truncated bool
		err       error
	)
	if path == "-" {
		if stdin == nil {
			return nil, false, fmt.Errorf("stdin is not available")
		}
		body, truncated, err = readLimited(stdin, maxBody)
	} else {
		body, truncated, err = readLimitedFile(path, maxBody)
	}
	if err != nil {
		return nil, false, err
	}

	if truncated {
		if lastNewline := bytes.LastIndexByte(body, '\n'); lastNewline >= 0 {
			body = body[:lastNewline+1]
		} else {
			body = nil
		}
	}
	return strings.Split(string(body), "\n"), truncated, nil
}
