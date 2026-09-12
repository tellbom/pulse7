package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// PUT /api/config writes only the user layer. Do not serialize this request
// into the session, audit, events or diagnostic output.
type apiConfigUpdate struct {
	BaseURL *string `json:"base_url"`
	Model   *string `json:"model"`
	APIKey  *string `json:"api_key"`
}

func validateAPIConnection(base, model string) error {
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("base_url must be an HTTP(S) endpoint without credentials, query or fragment")
	}
	if strings.TrimSpace(model) == "" {
		return errors.New("model is required")
	}
	return nil
}

func saveAPIConfig(path string, update apiConfigUpdate) error {
	doc, _, err := readConfigDocument(path)
	if err != nil {
		return errors.New("could not read global configuration")
	}
	if doc == nil {
		doc = map[string]json.RawMessage{}
	}
	for k, v := range map[string]*string{"base_url": update.BaseURL, "model": update.Model, "api_key": update.APIKey} {
		if v != nil {
			doc[k], err = json.Marshal(*v)
			if err != nil {
				return err
			}
		}
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return errors.New("could not encode global configuration")
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return errors.New("could not create global configuration directory")
	}
	f, err := os.CreateTemp(filepath.Dir(path), "config-*.tmp")
	if err != nil {
		return errors.New("could not create global configuration temporary file")
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(append(b, '\n')); err != nil {
		f.Close()
		return errors.New("could not write global configuration")
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return errors.New("could not sync global configuration")
	}
	if err = f.Close(); err != nil {
		return errors.New("could not close global configuration")
	}
	if err = os.Rename(name, path); err != nil {
		return errors.New("could not replace global configuration")
	}
	return nil
}

func apiSessionPath(dir, id string) (string, error) {
	if id == "" || id == "." || id == ".." || strings.ContainsAny(id, `/\:`) || strings.ContainsRune(id, 0) {
		return "", errors.New("invalid session id")
	}
	path := filepath.Join(dir, id+".jsonl")
	// session.id strips the standard sess- prefix; custom --session filenames
	// remain supported. Ambiguous identities are errors, not guessed matches.
	prefixed := filepath.Join(dir, "sess-"+id+".jsonl")
	_, plainErr := os.Stat(path)
	_, prefixedErr := os.Stat(prefixed)
	if plainErr == nil && prefixedErr == nil {
		return "", errors.New("ambiguous session id")
	}
	if os.IsNotExist(plainErr) && prefixedErr == nil {
		path = prefixed
	}

	root, _, _, err := resolveExistingPrefix(dir)
	if err != nil {
		return "", err
	}
	resolved, _, _, err := resolveExistingPrefix(path)
	if err != nil {
		return "", err
	}
	if requirePathWithin(root, resolved) != nil {
		return "", errors.New("session resolves outside session directory")
	}
	return resolved, nil
}

// Read persisted messages, not loadSession's synthesized tool results. The
// pagination endpoint must not repair or modify history as a read side effect.
func readAPISessionMessages(path string) ([]json.RawMessage, error) {
	if err := requireCompleteJSONL(path); err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rows := []json.RawMessage{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), maxSessionRecordBytes)
	line := 0
	for sc.Scan() {
		line++
		var record struct {
			Role      string          `json:"role"`
			System    string          `json:"system"`
			ToolCalls json.RawMessage `json:"tool_calls"`
		}
		if json.Unmarshal(sc.Bytes(), &record) != nil || record.Role == "" {
			return nil, fmt.Errorf("invalid session record at line %d", line)
		}
		if record.Role == "_meta" {
			if line != 1 {
				return nil, errors.New("invalid session metadata position")
			}
			continue
		}
		if record.Role == "_clear" {
			if record.System == "" {
				return nil, errors.New("invalid session clear boundary")
			}
			continue
		}
		if record.Role != "system" && record.Role != "user" && record.Role != "assistant" && record.Role != "tool" {
			return nil, fmt.Errorf("invalid session role at line %d", line)
		}
		var valid sessionMessageRecord
		if err := json.Unmarshal(sc.Bytes(), &valid); err != nil {
			return nil, fmt.Errorf("invalid session message at line %d", line)
		}
		rows = append(rows, append(json.RawMessage{}, sc.Bytes()...))
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return rows, nil
}

func decodeAPIJSON(body io.Reader, v interface{}) error {
	dec := json.NewDecoder(body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return errors.New("invalid JSON request")
	}
	var tail interface{}
	if err := dec.Decode(&tail); err != io.EOF {
		return errors.New("request must contain one JSON value")
	}
	return nil
}
