package app

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
)

type collector struct {
	mu     sync.Mutex
	seen   map[string]struct{}
	silent bool
	stdout io.Writer
}

func newCollector(stdout io.Writer, silent bool) *collector {
	return &collector{
		seen:   make(map[string]struct{}),
		silent: silent,
		stdout: stdout,
	}
}

func (c *collector) Add(value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.seen[value]; exists {
		return
	}
	c.seen[value] = struct{}{}
	if c.silent {
		fmt.Fprintln(c.stdout, value)
	}
}

func (c *collector) Sorted() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	values := make([]string, 0, len(c.seen))
	for value := range c.seen {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

func writeLines(path string, values []string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}

	writer := bufio.NewWriter(file)
	for _, value := range values {
		if _, err := fmt.Fprintln(writer, value); err != nil {
			file.Close()
			return err
		}
	}
	if err := writer.Flush(); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}
