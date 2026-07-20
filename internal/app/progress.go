package app

import (
	"fmt"
	"io"
	"sync"
)

type progress struct {
	mu      sync.Mutex
	writer  io.Writer
	enabled bool
}

func newProgress(writer io.Writer, enabled bool) *progress {
	return &progress{writer: writer, enabled: enabled}
}

func (p *progress) Stage(format string, args ...any) {
	if !p.enabled {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprintf(p.writer, "[+] "+format+"\n", args...)
}

func (p *progress) Warn(format string, args ...any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprintf(p.writer, "[!] "+format+"\n", args...)
}
