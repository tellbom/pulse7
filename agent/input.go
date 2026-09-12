package main

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"time"
)

type inputResult struct {
	line string
	err  error
}

type lineInput struct {
	results chan inputResult
}

func newLineInput(r io.Reader) *lineInput {
	in := &lineInput{results: make(chan inputResult)}
	go func() {
		defer close(in.results)
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 64*1024), 256*1024)
		for sc.Scan() {
			in.results <- inputResult{line: string(sc.Bytes())}
		}
		if err := sc.Err(); err != nil {
			in.results <- inputResult{err: err}
		}
	}()
	return in
}

func (in *lineInput) read(cancelled func() bool) (string, error) {
	if in == nil {
		return "", errors.New("input is not initialized")
	}
	tick := time.NewTicker(25 * time.Millisecond)
	defer tick.Stop()
	for {
		if cancelled != nil && cancelled() {
			return "", errInterrupted
		}
		select {
		case result, ok := <-in.results:
			if !ok {
				return "", io.EOF
			}
			if result.err != nil {
				return "", result.err
			}
			return strings.TrimSpace(decodeShellOutput([]byte(result.line))), nil
		case <-tick.C:
		}
	}
}
