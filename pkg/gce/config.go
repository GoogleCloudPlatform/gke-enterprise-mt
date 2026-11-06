// Package gce provides utilities for creating a GCE config and token source for a given provider config.
package gce

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const defaultPath = "/etc/gce.conf"

// TokenBody represents the body of the message to be sent to the TokenSource.
// In the context of GKE this includes the customer's project number and the Cluster ID.
type TokenBody struct {
	ProjectNumber int    `json:"projectNumber"`
	ClusterID     string `json:"clusterId"`
}

// Global represents attributes found in /etc/gce.conf.
type Global struct {
	// APIEndpoint is the GCE compute API endpoint to use. If this is blank,
	// then the default endpoint is used.
	APIEndpoint string `gcfg:"api-endpoint"`
	// NetworkProjectID refers to the project which owns the network being used.
	NetworkProjectID string `gcfg:"network-project-id"`
	NetworkName      string `gcfg:"network-name"`
	NetworkURL       string `gcfg:"network-url"`

	TokenURL     string `gcfg:"token-url"`
	TokenBodyRaw string `gcfg:"token-body"`
	TokenBody    *TokenBody
	ProjectID    string `gcfg:"project-id"`
	Regional     string `gcfg:"regional"`
}

// DeepCopy creates a deep copy of the Global configuration.
func (g *Global) DeepCopy() *Global {
	if g == nil {
		return nil
	}
	cp := *g
	if g.TokenBody != nil {
		cp.TokenBody = &TokenBody{}
		*cp.TokenBody = *g.TokenBody
	}
	return &cp
}

// Config envelopes Global to comply with the structure of gce.conf.
type Config struct {
	Global
}

// DeepCopy creates a deep copy of the Config.
func (c *Config) DeepCopy() *Config {
	if c == nil {
		return nil
	}
	return &Config{
		Global: *c.Global.DeepCopy(),
	}
}

// Read reads gce.conf and returns *Config.
// When pathToGCEConf is empty, this function assumes /etc/gce.conf.
func Read(pathToGCEConf string) (*Config, error) {
	if pathToGCEConf == "" {
		pathToGCEConf = defaultPath
	}

	r, err := os.Open(pathToGCEConf)
	if err != nil {
		return nil, fmt.Errorf("opening GCE config %q: %w", pathToGCEConf, err)
	}
	defer r.Close()
	return read(r)
}

func read(reader io.Reader) (*Config, error) {
	cfg := &Config{}
	scanner := bufio.NewScanner(reader)
	inGlobal := false
	for scanner.Scan() {
		line := strings.TrimSpace(stripInlineComment(scanner.Text()))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section := strings.TrimSpace(line[1 : len(line)-1])
			inGlobal = strings.EqualFold(section, "global")
			continue
		}
		if !inGlobal {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("malformed gce.conf line: %q", line)
		}
		key = strings.ToLower(strings.TrimSpace(key))
		val, _, err := unquoteJSON(strings.TrimSpace(val))
		if err != nil {
			return nil, fmt.Errorf("unquoting %s in gce.conf: %w", key, err)
		}
		switch key {
		case "api-endpoint":
			cfg.Global.APIEndpoint = val
		case "network-project-id":
			cfg.Global.NetworkProjectID = val
		case "network-name":
			cfg.Global.NetworkName = val
		case "network-url":
			cfg.Global.NetworkURL = val
		case "token-url":
			cfg.Global.TokenURL = val
		case "token-body":
			cfg.Global.TokenBodyRaw = val
		case "project-id":
			cfg.Global.ProjectID = val
		case "regional":
			cfg.Global.Regional = val
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading GCE config: %w", err)
	}

	if cfg.Global.TokenBodyRaw != "" {
		tb, err := parseTokenBody(cfg.Global.TokenBodyRaw)
		if err != nil {
			return nil, err
		}
		cfg.Global.TokenBody = tb
	}

	if strings.Contains(cfg.Global.NetworkName, "/") {
		cfg.Global.NetworkURL = cfg.Global.NetworkName
	}
	if cfg.Global.NetworkProjectID == "" {
		cfg.Global.NetworkProjectID = cfg.Global.ProjectID
	}

	return cfg, nil
}

// stripInlineComment removes unquoted '#' or ';' comments from a line.
func stripInlineComment(s string) string {
	inQuotes := false
	escaped := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' && inQuotes {
			escaped = true
			continue
		}
		if ch == '"' {
			inQuotes = !inQuotes
			continue
		}
		if !inQuotes && (ch == '#' || ch == ';') {
			return s[:i]
		}
	}
	return s
}

func unquoteJSON(raw string) (string, bool, error) {
	if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		unquoted, err := strconv.Unquote(raw)
		if err != nil {
			return "", true, fmt.Errorf("error unquoting TokenBody: %w", err)
		}
		return unquoted, true, nil
	}
	return raw, false, nil
}

func parseTokenBody(raw string) (*TokenBody, error) {
	unquoted, _, err := unquoteJSON(raw)
	if err != nil {
		return nil, err
	}
	tb := &TokenBody{}
	if err := json.Unmarshal([]byte(unquoted), tb); err != nil {
		return nil, fmt.Errorf("error unmarshaling TokenBody: %w", err)
	}
	return tb, nil
}
