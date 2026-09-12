// Copyright (c) MemoryStore 2026
// SPDX-License-Identifier: MPL-2.0

package testfake

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MemoryStore/terraform-provider-openaiagents/internal/client"
)

// Server is an in-memory Agents API used by offline tests.
type Server struct {
	mu          sync.Mutex
	agents      map[string]*agentRecord
	templates   map[string]*templateRecord
	vaults      map[string]*vaultRecord
	credentials map[string]*credentialRecord
	seq         atomic.Int64
	Requests    []Request
	base        string
	srv         *http.Server
	ln          net.Listener
}

// Request is a captured inbound HTTP request.
type Request struct {
	Method  string
	Path    string
	Headers http.Header
	Body    []byte
}

type agentRecord struct {
	client.Agent
	LastWrite []byte
}

type templateRecord struct {
	Public    client.EnvironmentTemplate
	Env       map[string]string
	Setup     []client.SetupCommand
	Files     []client.TemplateFile
	Skills    []client.TemplateSkill
	Plugins   []client.TemplatePlugin
	LastWrite []byte
}

type vaultRecord struct {
	client.Vault
}

type credentialRecord struct {
	Public                client.Credential
	Token                 string
	AccessToken           string
	RefreshToken          string
	ClientSecret          string
	RefreshScope          string
	RefreshResource       string
	TokenEndpoint         string
	ClientID              string
	TokenEndpointAuthType string
	AuthType              string
	MCPServerURL          string
	VaultID               string
	RotateCount           int
	LastAuth              []byte
}

// New constructs an in-memory fake without listening.
func New() *Server {
	return &Server{
		agents:      map[string]*agentRecord{},
		templates:   map[string]*templateRecord{},
		vaults:      map[string]*vaultRecord{},
		credentials: map[string]*credentialRecord{},
	}
}

// Listen binds a working loopback server. Call Close when finished.
func Listen() (*Server, error) {
	s := New()
	var lastErr error
	for attempt := 0; attempt < 40; attempt++ {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			lastErr = err
			continue
		}
		s.ln = ln
		s.base = "http://" + ln.Addr().String()
		srv := &http.Server{Handler: s}
		s.srv = srv
		go func() { _ = srv.Serve(ln) }()
		ok := false
		deadline := time.Now().Add(300 * time.Millisecond)
		for time.Now().Before(deadline) {
			c, err := net.DialTimeout("tcp", ln.Addr().String(), 50*time.Millisecond)
			if err == nil {
				_ = c.Close()
				ok = true
				break
			}
			lastErr = err
			time.Sleep(10 * time.Millisecond)
		}
		if ok {
			return s, nil
		}
		_ = srv.Close()
		_ = ln.Close()
		s.srv = nil
		s.ln = nil
	}
	return nil, fmt.Errorf("fake API server never accepted connections: %v", lastErr)
}

// Close stops the server.
func (s *Server) Close() error {
	if s.srv != nil {
		return s.srv.Close()
	}
	if s.ln != nil {
		return s.ln.Close()
	}
	return nil
}

// Start listens on loopback and serves the Agents endpoints.
func Start(t testing.TB) *Server {
	t.Helper()
	s, err := Listen()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.serve(w, r)
}

// URL is the fake origin, including /v1.
func (s *Server) URL() string {
	if s.base == "" {
		return "http://127.0.0.1/v1"
	}
	return s.base + "/v1"
}

func (s *Server) nextID(prefix string) string {
	n := s.seq.Add(1)
	return fmt.Sprintf("%s_%d", prefix, n)
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	s.mu.Lock()
	headers := r.Header.Clone()
	s.Requests = append(s.Requests, Request{Method: r.Method, Path: r.URL.Path, Headers: headers, Body: append([]byte(nil), body...)})
	s.mu.Unlock()

	if r.Header.Get("OpenAI-Beta") != "agents=v1" {
		writeError(w, http.StatusBadRequest, "missing OpenAI-Beta: agents=v1")
		return
	}
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Authorization")), "bearer ") {
		writeError(w, http.StatusUnauthorized, "missing bearer token")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/v1")
	parts := splitPath(path)
	switch {
	case len(parts) == 1 && parts[0] == "agents" && r.Method == http.MethodPost:
		s.createAgent(w, body)
	case len(parts) == 2 && parts[0] == "agents" && parts[1] != "environments" && r.Method == http.MethodGet:
		s.getAgent(w, parts[1])
	case len(parts) == 2 && parts[0] == "agents" && parts[1] != "environments" && r.Method == http.MethodPost:
		s.updateAgent(w, parts[1], body)
	case len(parts) == 2 && parts[0] == "agents" && parts[1] != "environments" && r.Method == http.MethodDelete:
		s.deleteAgent(w, parts[1])
	case len(parts) == 3 && parts[0] == "agents" && parts[1] == "environments" && parts[2] == "templates" && r.Method == http.MethodPost:
		s.createTemplate(w, body)
	case len(parts) == 4 && parts[0] == "agents" && parts[1] == "environments" && parts[2] == "templates" && r.Method == http.MethodGet:
		s.getTemplate(w, parts[3])
	case len(parts) == 4 && parts[0] == "agents" && parts[1] == "environments" && parts[2] == "templates" && r.Method == http.MethodPost:
		s.updateTemplate(w, parts[3], body)
	case len(parts) == 4 && parts[0] == "agents" && parts[1] == "environments" && parts[2] == "templates" && r.Method == http.MethodDelete:
		s.deleteTemplate(w, parts[3])
	case len(parts) == 1 && parts[0] == "vaults" && r.Method == http.MethodPost:
		s.createVault(w, body)
	case len(parts) == 2 && parts[0] == "vaults" && r.Method == http.MethodGet:
		s.getVault(w, parts[1])
	case len(parts) == 2 && parts[0] == "vaults" && r.Method == http.MethodDelete:
		s.deleteVault(w, parts[1])
	case len(parts) == 3 && parts[0] == "vaults" && parts[2] == "credentials" && r.Method == http.MethodPost:
		s.createCredential(w, parts[1], body)
	case len(parts) == 4 && parts[0] == "vaults" && parts[2] == "credentials" && r.Method == http.MethodGet:
		s.getCredential(w, parts[1], parts[3])
	case len(parts) == 4 && parts[0] == "vaults" && parts[2] == "credentials" && r.Method == http.MethodPost:
		s.rotateCredential(w, parts[1], parts[3], body)
	case len(parts) == 4 && parts[0] == "vaults" && parts[2] == "credentials" && r.Method == http.MethodDelete:
		s.deleteCredential(w, parts[1], parts[3])
	default:
		writeError(w, http.StatusNotFound, "unknown path")
	}
}

func splitPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-ID", "req_fake")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{"message": message, "type": "invalid_request_error"},
	})
}

func (s *Server) createAgent(w http.ResponseWriter, body []byte) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	model, _ := raw["model"].(string)
	if model == "" {
		writeError(w, http.StatusBadRequest, "model is required")
		return
	}
	now := time.Now().Unix()
	rec := &agentRecord{Agent: client.Agent{
		ID:          s.nextID("agent"),
		Object:      "agent",
		CreatedAt:   now,
		UpdatedAt:   now,
		Model:       model,
		Metadata:    map[string]string{},
		ServiceTier: "auto",
		MultiAgent:  &client.MultiAgent{Enabled: false},
		Reasoning:   &client.Reasoning{},
		Text:        &client.Text{Format: &client.TextFormat{Type: "text"}, Verbosity: strPtr("medium")},
		Tools:       []json.RawMessage{},
	}}
	rec.LastWrite = append([]byte(nil), body...)
	applyAgent(rec, raw, true)
	normalizeAgent(rec)
	s.mu.Lock()
	s.agents[rec.ID] = rec
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, rec.Agent)
}

func (s *Server) getAgent(w http.ResponseWriter, id string) {
	s.mu.Lock()
	rec, ok := s.agents[id]
	s.mu.Unlock()
	if !ok {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	writeJSON(w, http.StatusOK, rec.Agent)
}

func (s *Server) updateAgent(w http.ResponseWriter, id string, body []byte) {
	s.mu.Lock()
	rec, ok := s.agents[id]
	s.mu.Unlock()
	if !ok {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	rec.LastWrite = append([]byte(nil), body...)
	applyAgent(rec, raw, false)
	normalizeAgent(rec)
	rec.UpdatedAt = time.Now().Unix()
	writeJSON(w, http.StatusOK, rec.Agent)
}

func (s *Server) deleteAgent(w http.ResponseWriter, id string) {
	s.mu.Lock()
	_, ok := s.agents[id]
	delete(s.agents, id)
	s.mu.Unlock()
	if !ok {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	writeJSON(w, http.StatusOK, client.Deleted{ID: id, Object: "agent.deleted", Deleted: true})
}

func applyAgent(rec *agentRecord, raw map[string]any, create bool) {
	if v, ok := raw["model"].(string); ok && v != "" {
		rec.Model = v
	}
	if v, exists := raw["instructions"]; exists {
		rec.Instructions = stringOrNil(v)
	}
	if v, exists := raw["name"]; exists {
		rec.Name = stringOrNil(v)
	}
	if v, exists := raw["metadata"]; exists {
		rec.Metadata = stringMapOrEmpty(v)
	} else if create {
		rec.Metadata = map[string]string{}
	}
	if v, exists := raw["service_tier"]; exists {
		if s, ok := v.(string); ok && s != "" {
			rec.ServiceTier = s
		}
	}
	if v, exists := raw["multi_agent"]; exists {
		if v == nil {
			rec.MultiAgent = &client.MultiAgent{Enabled: false}
		} else {
			b, _ := json.Marshal(v)
			var ma client.MultiAgent
			_ = json.Unmarshal(b, &ma)
			rec.MultiAgent = &ma
		}
	}
	if v, exists := raw["reasoning"]; exists && v != nil {
		b, _ := json.Marshal(v)
		var r client.Reasoning
		_ = json.Unmarshal(b, &r)
		rec.Reasoning = &r
	}
	if v, exists := raw["text"]; exists && v != nil {
		b, _ := json.Marshal(v)
		var t client.Text
		_ = json.Unmarshal(b, &t)
		rec.Text = &t
	}
	if v, exists := raw["tools"]; exists {
		if v == nil {
			rec.Tools = []json.RawMessage{}
		} else {
			b, _ := json.Marshal(v)
			var tools []json.RawMessage
			_ = json.Unmarshal(b, &tools)
			rec.Tools = tools
		}
	}
}

func normalizeAgent(rec *agentRecord) {
	if rec.ServiceTier == "" {
		rec.ServiceTier = "auto"
	}
	if rec.MultiAgent == nil {
		rec.MultiAgent = &client.MultiAgent{Enabled: false}
	}
	if rec.MultiAgent.Enabled && rec.MultiAgent.MaxConcurrentSubagents == nil {
		six := int64(6)
		rec.MultiAgent.MaxConcurrentSubagents = &six
	}
	if rec.Metadata == nil {
		rec.Metadata = map[string]string{}
	}
	out := make([]json.RawMessage, 0, len(rec.Tools))
	for _, raw := range rec.Tools {
		out = append(out, normalizeTool(raw))
	}
	rec.Tools = out
}

func normalizeTool(raw json.RawMessage) json.RawMessage {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return raw
	}
	typ, _ := obj["type"].(string)
	switch typ {
	case "function":
		if _, ok := obj["defer_loading"]; !ok {
			obj["defer_loading"] = false
		}
	case "programmatic_tool_calling":
		if _, ok := obj["enabled"]; !ok {
			obj["enabled"] = true
		}
	case "mcp":
		if _, ok := obj["required"]; !ok {
			obj["required"] = false
		}
	case "web_search":
		if _, ok := obj["type"]; !ok {
			obj["type"] = "web_search"
		}
		if _, ok := obj["context_size"]; !ok {
			obj["context_size"] = "medium"
		}
		if _, ok := obj["mode"]; !ok {
			obj["mode"] = "live"
		}
	}
	b, err := json.Marshal(obj)
	if err != nil {
		return raw
	}
	return b
}

func (s *Server) createTemplate(w http.ResponseWriter, body []byte) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	now := time.Now().Unix()
	rec := &templateRecord{
		Public: client.EnvironmentTemplate{
			ID:                    s.nextID("envtpl"),
			Object:                "environment.template",
			CreatedAt:             now,
			UpdatedAt:             now,
			CapabilityDirectories: []string{},
		},
	}
	rec.LastWrite = append([]byte(nil), body...)
	applyTemplate(rec, raw)
	s.mu.Lock()
	s.templates[rec.Public.ID] = rec
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, publicTemplate(rec))
}

func (s *Server) getTemplate(w http.ResponseWriter, id string) {
	s.mu.Lock()
	rec, ok := s.templates[id]
	s.mu.Unlock()
	if !ok {
		writeError(w, http.StatusNotFound, "template not found")
		return
	}
	writeJSON(w, http.StatusOK, publicTemplate(rec))
}

func (s *Server) updateTemplate(w http.ResponseWriter, id string, body []byte) {
	s.mu.Lock()
	rec, ok := s.templates[id]
	s.mu.Unlock()
	if !ok {
		writeError(w, http.StatusNotFound, "template not found")
		return
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	rec.LastWrite = append([]byte(nil), body...)
	applyTemplate(rec, raw)
	rec.Public.UpdatedAt = time.Now().Unix()
	writeJSON(w, http.StatusOK, publicTemplate(rec))
}

func (s *Server) deleteTemplate(w http.ResponseWriter, id string) {
	s.mu.Lock()
	_, ok := s.templates[id]
	delete(s.templates, id)
	s.mu.Unlock()
	if !ok {
		writeError(w, http.StatusNotFound, "template not found")
		return
	}
	writeJSON(w, http.StatusOK, client.Deleted{ID: id, Object: "environment.template.deleted", Deleted: true})
}

func applyTemplate(rec *templateRecord, raw map[string]any) {
	if v, exists := raw["name"]; exists {
		rec.Public.Name = stringOrNil(v)
	}
	if v, exists := raw["capability_directories"]; exists {
		rec.Public.CapabilityDirectories = stringSlice(v)
	}
	if v, exists := raw["network"]; exists && v != nil {
		b, _ := json.Marshal(v)
		var n client.Network
		_ = json.Unmarshal(b, &n)
		rec.Public.Network = &n
	}
	if v, exists := raw["packages"]; exists && v != nil {
		b, _ := json.Marshal(v)
		var p client.Packages
		_ = json.Unmarshal(b, &p)
		rec.Public.Packages = &p
	}
	if v, exists := raw["env"]; exists {
		rec.Env = stringMapOrEmpty(v)
	}
	if v, exists := raw["setup_commands"]; exists {
		b, _ := json.Marshal(v)
		var cmds []client.SetupCommand
		_ = json.Unmarshal(b, &cmds)
		rec.Setup = cmds
	}
	if v, exists := raw["files"]; exists {
		b, _ := json.Marshal(v)
		var files []client.TemplateFile
		_ = json.Unmarshal(b, &files)
		rec.Files = files
	}
	if v, exists := raw["skills"]; exists {
		b, _ := json.Marshal(v)
		var skills []client.TemplateSkill
		_ = json.Unmarshal(b, &skills)
		rec.Skills = skills
	}
	if v, exists := raw["plugins"]; exists {
		b, _ := json.Marshal(v)
		var plugins []client.TemplatePlugin
		_ = json.Unmarshal(b, &plugins)
		rec.Plugins = plugins
	}
}

func publicTemplate(rec *templateRecord) client.EnvironmentTemplate {
	out := rec.Public
	if rec.Env != nil {
		keys := make([]string, 0, len(rec.Env))
		for k := range rec.Env {
			keys = append(keys, k)
		}
		out.EnvKeys = keys
	}
	out.Files = make([]client.TemplateFile, len(rec.Files))
	for i, f := range rec.Files {
		out.Files[i] = client.TemplateFile{Type: f.Type, Path: f.Path, FileID: f.FileID}
	}
	out.Skills = make([]client.TemplateSkill, len(rec.Skills))
	for i, sk := range rec.Skills {
		cp := sk
		if cp.Source != nil {
			src := *cp.Source
			src.Data = ""
			cp.Source = &src
		}
		out.Skills[i] = cp
	}
	out.Plugins = make([]client.TemplatePlugin, len(rec.Plugins))
	for i, p := range rec.Plugins {
		cp := p
		if cp.Source != nil {
			src := *cp.Source
			src.Data = ""
			cp.Source = &src
		}
		out.Plugins[i] = cp
	}
	return out
}

func (s *Server) createVault(w http.ResponseWriter, body []byte) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	rec := &vaultRecord{Vault: client.Vault{
		ID:        s.nextID("vault"),
		Object:    "vault",
		CreatedAt: time.Now().Unix(),
		Name:      stringOrNil(raw["name"]),
		Metadata:  stringMapOrEmpty(raw["metadata"]),
	}}
	s.mu.Lock()
	s.vaults[rec.ID] = rec
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, rec.Vault)
}

func (s *Server) getVault(w http.ResponseWriter, id string) {
	s.mu.Lock()
	rec, ok := s.vaults[id]
	s.mu.Unlock()
	if !ok {
		writeError(w, http.StatusNotFound, "vault not found")
		return
	}
	writeJSON(w, http.StatusOK, rec.Vault)
}

func (s *Server) deleteVault(w http.ResponseWriter, id string) {
	s.mu.Lock()
	_, ok := s.vaults[id]
	delete(s.vaults, id)
	for cid, cred := range s.credentials {
		if cred.VaultID == id {
			delete(s.credentials, cid)
		}
	}
	s.mu.Unlock()
	if !ok {
		writeError(w, http.StatusNotFound, "vault not found")
		return
	}
	writeJSON(w, http.StatusOK, client.Deleted{ID: id, Object: "vault.deleted", Deleted: true})
}

func (s *Server) createCredential(w http.ResponseWriter, vaultID string, body []byte) {
	s.mu.Lock()
	_, ok := s.vaults[vaultID]
	s.mu.Unlock()
	if !ok {
		writeError(w, http.StatusNotFound, "vault not found")
		return
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	auth, _ := raw["auth"].(map[string]any)
	if auth == nil {
		writeError(w, http.StatusBadRequest, "auth is required")
		return
	}
	authType, _ := auth["type"].(string)
	mcpURL, _ := auth["mcp_server_url"].(string)
	now := time.Now().Unix()
	id := s.nextID("cred")
	rec := &credentialRecord{
		Public: client.Credential{
			ID:           id,
			Object:       "vault.credential",
			CreatedAt:    now,
			UpdatedAt:    now,
			VaultID:      vaultID,
			Name:         stringOrNil(raw["name"]),
			AuthType:     authType,
			MCPServerURL: mcpURL,
		},
		AuthType:     authType,
		MCPServerURL: mcpURL,
		VaultID:      vaultID,
	}
	if token, ok := auth["token"].(string); ok {
		rec.Token = token
	}
	if at, ok := auth["access_token"].(string); ok {
		rec.AccessToken = at
	}
	if exp, ok := auth["expires_at"].(string); ok {
		rec.Public.ExpiresAt = &exp
	}
	if refresh, exists := auth["refresh"]; exists {
		applyCredentialRefresh(rec, refresh)
	}
	if authJSON, err := json.Marshal(auth); err == nil {
		rec.LastAuth = authJSON
	}
	s.mu.Lock()
	s.credentials[id] = rec
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, publicCredential(rec))
}

func (s *Server) getCredential(w http.ResponseWriter, vaultID, id string) {
	s.mu.Lock()
	rec, ok := s.credentials[id]
	s.mu.Unlock()
	if !ok || rec.VaultID != vaultID {
		writeError(w, http.StatusNotFound, "credential not found")
		return
	}
	writeJSON(w, http.StatusOK, publicCredential(rec))
}

func (s *Server) rotateCredential(w http.ResponseWriter, vaultID, id string, body []byte) {
	s.mu.Lock()
	rec, ok := s.credentials[id]
	s.mu.Unlock()
	if !ok || rec.VaultID != vaultID {
		writeError(w, http.StatusNotFound, "credential not found")
		return
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	auth, _ := raw["auth"].(map[string]any)
	if auth == nil {
		writeError(w, http.StatusBadRequest, "auth is required")
		return
	}
	rec.RotateCount++
	if token, exists := auth["token"]; exists {
		if token == nil {
			rec.Token = ""
		} else if s, ok := token.(string); ok {
			rec.Token = s
		}
	}
	if at, exists := auth["access_token"]; exists {
		if at == nil {
			rec.AccessToken = ""
		} else if s, ok := at.(string); ok {
			rec.AccessToken = s
		}
	}
	if exp, exists := auth["expires_at"]; exists {
		rec.Public.ExpiresAt = stringOrNil(exp)
	}
	if refresh, exists := auth["refresh"]; exists {
		applyCredentialRefresh(rec, refresh)
	}
	if authJSON, err := json.Marshal(auth); err == nil {
		rec.LastAuth = authJSON
	}
	rec.Public.UpdatedAt = time.Now().Unix()
	writeJSON(w, http.StatusOK, publicCredential(rec))
}

// publicCredential is the documented retrieve/create envelope: nested auth,
// no secret values, no flattened auth_type/mcp_server_url at the top level.
func publicCredential(rec *credentialRecord) map[string]any {
	auth := map[string]any{
		"type":           rec.AuthType,
		"mcp_server_url": rec.MCPServerURL,
	}
	if rec.Public.ExpiresAt != nil {
		auth["expires_at"] = *rec.Public.ExpiresAt
	}
	if rec.TokenEndpoint != "" || rec.ClientID != "" || rec.RefreshScope != "" || rec.RefreshResource != "" {
		refresh := map[string]any{}
		if rec.TokenEndpoint != "" {
			refresh["token_endpoint"] = rec.TokenEndpoint
		}
		if rec.ClientID != "" {
			refresh["client_id"] = rec.ClientID
		}
		if rec.RefreshScope != "" {
			refresh["scope"] = rec.RefreshScope
		}
		if rec.RefreshResource != "" {
			refresh["resource"] = rec.RefreshResource
		}
		if rec.TokenEndpointAuthType != "" {
			refresh["token_endpoint_auth"] = map[string]any{"type": rec.TokenEndpointAuthType}
		}
		auth["refresh"] = refresh
	}
	out := map[string]any{
		"id":         rec.Public.ID,
		"object":     rec.Public.Object,
		"created_at": rec.Public.CreatedAt,
		"updated_at": rec.Public.UpdatedAt,
		"vault_id":   rec.VaultID,
		"auth":       auth,
	}
	if rec.Public.Name != nil {
		out["name"] = *rec.Public.Name
	}
	return out
}

func (s *Server) deleteCredential(w http.ResponseWriter, vaultID, id string) {
	s.mu.Lock()
	rec, ok := s.credentials[id]
	if ok && rec.VaultID == vaultID {
		delete(s.credentials, id)
	} else {
		ok = false
	}
	s.mu.Unlock()
	if !ok {
		writeError(w, http.StatusNotFound, "credential not found")
		return
	}
	writeJSON(w, http.StatusOK, client.Deleted{ID: id, Object: "vault.credential.deleted", Deleted: true})
}

// LastRequest returns the most recent captured request.
func (s *Server) LastRequest() Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.Requests) == 0 {
		return Request{}
	}
	return s.Requests[len(s.Requests)-1]
}

// RequestCount returns how many requests the fake API has received.
func (s *Server) RequestCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.Requests)
}

// AgentLastWrite returns the raw JSON body of the last create or update for an agent.
// Tests use this to prove request mapping; it is the inbound payload, not the normalized store.
func (s *Server) AgentLastWrite(id string) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.agents[id]
	if !ok {
		return nil
	}
	return append([]byte(nil), rec.LastWrite...)
}

// CredentialLastAuth returns the raw auth object from the last create or rotate.
func (s *Server) CredentialLastAuth(id string) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.credentials[id]
	if !ok {
		return nil
	}
	return append([]byte(nil), rec.LastAuth...)
}

// StoredOAuthRefresh is non-secret OAuth refresh material stored by the fake.
type StoredOAuthRefresh struct {
	Scope         string
	Resource      string
	TokenEndpoint string
	ClientID      string
}

// StoredRefresh returns OAuth refresh grant fields stored for a credential.
func (s *Server) StoredRefresh(id string) StoredOAuthRefresh {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.credentials[id]
	if !ok {
		return StoredOAuthRefresh{}
	}
	return StoredOAuthRefresh{
		Scope:         rec.RefreshScope,
		Resource:      rec.RefreshResource,
		TokenEndpoint: rec.TokenEndpoint,
		ClientID:      rec.ClientID,
	}
}

// Agent returns a stored agent by ID.
func (s *Server) Agent(id string) (client.Agent, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.agents[id]
	if !ok {
		return client.Agent{}, false
	}
	return rec.Agent, true
}

// DeleteAgent removes an agent, simulating external deletion.
func (s *Server) DeleteAgent(id string) {
	s.mu.Lock()
	delete(s.agents, id)
	s.mu.Unlock()
}

// SetAgentName mutates a stored agent name, simulating external drift.
func (s *Server) SetAgentName(id, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rec, ok := s.agents[id]; ok {
		n := name
		rec.Name = &n
	}
}

// SetTemplateName mutates a stored template name, simulating external drift.
func (s *Server) SetTemplateName(id, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rec, ok := s.templates[id]; ok {
		n := name
		rec.Public.Name = &n
	}
}

// DeleteVault simulates an external vault deletion, cascading credentials.
func (s *Server) DeleteVault(id string) {
	s.mu.Lock()
	delete(s.vaults, id)
	for cid, cred := range s.credentials {
		if cred.VaultID == id {
			delete(s.credentials, cid)
		}
	}
	s.mu.Unlock()
}

// CredentialRotateCount returns how many rotation requests a credential received.
func (s *Server) CredentialRotateCount(id string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rec, ok := s.credentials[id]; ok {
		return rec.RotateCount
	}
	return 0
}

// StoredToken returns the secret stored for a credential. Tests use this to
// prove rotation; it is never returned over HTTP.
func (s *Server) StoredToken(id string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rec, ok := s.credentials[id]; ok {
		return rec.Token
	}
	return ""
}

func applyCredentialRefresh(rec *credentialRecord, refresh any) {
	if refresh == nil {
		rec.RefreshToken = ""
		rec.ClientSecret = ""
		rec.RefreshScope = ""
		rec.RefreshResource = ""
		rec.TokenEndpoint = ""
		rec.ClientID = ""
		rec.TokenEndpointAuthType = ""
		return
	}
	m, ok := refresh.(map[string]any)
	if !ok {
		return
	}
	if v, ok := m["refresh_token"].(string); ok {
		rec.RefreshToken = v
	}
	if v, exists := m["scope"]; exists {
		if s, ok := v.(string); ok {
			rec.RefreshScope = s
		} else if v == nil {
			rec.RefreshScope = ""
		}
	}
	if v, exists := m["resource"]; exists {
		if s, ok := v.(string); ok {
			rec.RefreshResource = s
		} else if v == nil {
			rec.RefreshResource = ""
		}
	}
	if v, ok := m["token_endpoint"].(string); ok {
		rec.TokenEndpoint = v
	}
	if v, ok := m["client_id"].(string); ok {
		rec.ClientID = v
	}
	if tea, ok := m["token_endpoint_auth"].(map[string]any); ok {
		if t, ok := tea["type"].(string); ok {
			rec.TokenEndpointAuthType = t
		}
		if cs, ok := tea["client_secret"].(string); ok {
			rec.ClientSecret = cs
		}
	} else if _, exists := m["token_endpoint_auth"]; exists && m["token_endpoint_auth"] == nil {
		rec.TokenEndpointAuthType = ""
		rec.ClientSecret = ""
	}
}

func stringOrNil(v any) *string {
	if v == nil {
		return nil
	}
	s, ok := v.(string)
	if !ok {
		return nil
	}
	return &s
}

func strPtr(s string) *string { return &s }

func stringMapOrEmpty(v any) map[string]string {
	out := map[string]string{}
	m, ok := v.(map[string]any)
	if !ok {
		return out
	}
	for k, val := range m {
		if s, ok := val.(string); ok {
			out[k] = s
		}
	}
	return out
}

func stringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return []string{}
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// TemplateLastWrite returns the raw JSON body of the last create or update for a template.
// SetTemplateCapabilityDirectories overwrites stored capability directories.
// Tests use a non-nil empty slice to model an API that returned [].
func (s *Server) SetTemplateCapabilityDirectories(id string, dirs []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rec, ok := s.templates[id]; ok {
		if dirs == nil {
			rec.Public.CapabilityDirectories = nil
			return
		}
		cp := make([]string, len(dirs))
		copy(cp, dirs)
		rec.Public.CapabilityDirectories = cp
	}
}

func (s *Server) TemplateLastWrite(id string) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.templates[id]
	if !ok {
		return nil
	}
	return append([]byte(nil), rec.LastWrite...)
}

// TemplateHasEnv reports whether confidential env values were stored.
func (s *Server) TemplateHasEnv(id, key, value string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.templates[id]
	if !ok || rec.Env == nil {
		return false
	}
	return rec.Env[key] == value
}

func (s *Server) TemplateSetupCwd(id string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.templates[id]
	if !ok || len(rec.Setup) == 0 {
		return ""
	}
	return rec.Setup[0].Cwd
}

func (s *Server) TemplateEnvKeys(id string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.templates[id]
	if !ok || rec.Env == nil {
		return nil
	}
	out := make([]string, 0, len(rec.Env))
	for k := range rec.Env {
		out = append(out, k)
	}
	return out
}

func (s *Server) TemplateFilePath(id string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.templates[id]
	if !ok || len(rec.Files) == 0 {
		return ""
	}
	return rec.Files[0].Path
}

func (s *Server) TemplateSetupCommand(id string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.templates[id]
	if !ok || len(rec.Setup) == 0 {
		return ""
	}
	return rec.Setup[0].Command
}

// TemplateFileData returns stored inline file bytes.
func (s *Server) TemplateFileData(id string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.templates[id]
	if !ok || len(rec.Files) == 0 {
		return ""
	}
	return rec.Files[0].Data
}
