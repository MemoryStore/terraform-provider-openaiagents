// Copyright (c) MemoryStore 2026
// SPDX-License-Identifier: MPL-2.0

package client

import "encoding/json"

// Agent is the saved Agents API object.
type Agent struct {
	ID           string            `json:"id"`
	Object       string            `json:"object"`
	CreatedAt    int64             `json:"created_at"`
	UpdatedAt    int64             `json:"updated_at"`
	Model        string            `json:"model"`
	Instructions *string           `json:"instructions"`
	Name         *string           `json:"name"`
	Metadata     map[string]string `json:"metadata"`
	MultiAgent   *MultiAgent       `json:"multi_agent"`
	Reasoning    *Reasoning        `json:"reasoning"`
	ServiceTier  string            `json:"service_tier"`
	Text         *Text             `json:"text"`
	Tools        []json.RawMessage `json:"tools"`
}

// MultiAgent is persisted multi-agent configuration.
type MultiAgent struct {
	Enabled                bool   `json:"enabled"`
	MaxConcurrentSubagents *int64 `json:"max_concurrent_subagents"`
}

// Reasoning is persisted reasoning configuration.
type Reasoning struct {
	Effort  *string `json:"effort"`
	Summary *string `json:"summary"`
}

// Text is persisted text output configuration.
type Text struct {
	Format    *TextFormat `json:"format"`
	Verbosity *string     `json:"verbosity"`
}

// TextFormat is a text or json_schema output format.
type TextFormat struct {
	Type   string          `json:"type"`
	Schema json.RawMessage `json:"schema,omitempty"`
}

// AgentWrite is the caller-supplied saved-agent configuration.
type AgentWrite struct {
	Model        string
	Instructions Optional[string]
	Name         Optional[string]
	Metadata     Optional[map[string]string]
	MultiAgent   Optional[MultiAgent]
	Reasoning    Optional[Reasoning]
	ServiceTier  Optional[string]
	Text         Optional[Text]
	Tools        Optional[[]json.RawMessage]
}

// Optional is a tri-state value: omitted, JSON null, or a concrete value.
type Optional[T any] struct {
	Present bool
	Null    bool
	Value   T
}

// Set returns an Optional holding v.
func Set[T any](v T) Optional[T] {
	return Optional[T]{Present: true, Value: v}
}

// Null returns an Optional that marshals as JSON null.
func Null[T any]() Optional[T] {
	return Optional[T]{Present: true, Null: true}
}

func (o Optional[T]) apply(body map[string]any, key string) {
	if !o.Present {
		return
	}
	if o.Null {
		body[key] = nil
		return
	}
	body[key] = o.Value
}

// EnvironmentTemplate is the saved hosted environment template.
type EnvironmentTemplate struct {
	ID                    string             `json:"id"`
	Object                string             `json:"object"`
	CreatedAt             int64              `json:"created_at"`
	UpdatedAt             int64              `json:"updated_at"`
	Name                  *string            `json:"name"`
	CapabilityDirectories []string           `json:"capability_directories"`
	Network               *Network           `json:"network"`
	Packages              *Packages          `json:"packages"`
	Files                 []TemplateFile     `json:"files"`
	Skills                []TemplateSkill    `json:"skills"`
	Plugins               []TemplatePlugin   `json:"plugins"`
	EnvKeys               []string           `json:"env_keys,omitempty"`
	SetupCommandMetadata  []SetupCommandMeta `json:"setup_commands,omitempty"`
}

// Network is hosted-environment network policy.
type Network struct {
	Access         string   `json:"access,omitempty"`
	AllowedDomains []string `json:"allowed_domains,omitempty"`
}

// Packages lists packages to install in a hosted environment.
type Packages struct {
	Python []string `json:"python,omitempty"`
	System []string `json:"system,omitempty"`
	NPM    []string `json:"npm,omitempty"`
}

// TemplateFile is a template file entry. Data is write-only.
type TemplateFile struct {
	Type   string `json:"type"`
	Path   string `json:"path"`
	FileID string `json:"file_id,omitempty"`
	Data   string `json:"data,omitempty"`
}

// TemplateSkill is an inline or referenced skill.
type TemplateSkill struct {
	Type        string         `json:"type"`
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	SkillID     string         `json:"skill_id,omitempty"`
	Version     string         `json:"version,omitempty"`
	Source      *ArchiveSource `json:"source,omitempty"`
}

// TemplatePlugin is an inline plugin archive.
type TemplatePlugin struct {
	Type        string         `json:"type"`
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Source      *ArchiveSource `json:"source,omitempty"`
}

// ArchiveSource is a base64 archive payload.
type ArchiveSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type,omitempty"`
	Data      string `json:"data,omitempty"`
}

// SetupCommand is a hosted setup command. Command is confidential on readback.
type SetupCommand struct {
	Command string `json:"command"`
	Cwd     string `json:"cwd,omitempty"`
}

// SetupCommandMeta is the non-confidential remainder of a setup command.
type SetupCommandMeta struct {
	Cwd string `json:"cwd,omitempty"`
}

// TemplateWrite is the caller-supplied template configuration.
type TemplateWrite struct {
	Name                  Optional[string]
	CapabilityDirectories Optional[[]string]
	Network               Optional[Network]
	Packages              Optional[Packages]
	Files                 Optional[[]TemplateFile]
	Skills                Optional[[]TemplateSkill]
	Plugins               Optional[[]TemplatePlugin]
	Env                   Optional[map[string]string]
	SetupCommands         Optional[[]SetupCommand]
	SendConfidential      bool
}

// Vault is a credential vault.
type Vault struct {
	ID        string            `json:"id"`
	Object    string            `json:"object"`
	CreatedAt int64             `json:"created_at"`
	Name      *string           `json:"name"`
	Metadata  map[string]string `json:"metadata"`
}

// VaultWrite is vault create input. Vaults have no update API.
type VaultWrite struct {
	Name     Optional[string]
	Metadata Optional[map[string]string]
}

// Credential is vault credential metadata. Secrets are never populated.
// The Agents API returns authentication as a nested `auth` object
// (`auth.type`, `auth.mcp_server_url`, `auth.expires_at`); UnmarshalJSON
// lifts those into AuthType, MCPServerURL, and ExpiresAt.
type Credential struct {
	ID           string            `json:"id"`
	Object       string            `json:"object"`
	CreatedAt    int64             `json:"created_at"`
	UpdatedAt    int64             `json:"updated_at"`
	VaultID      string            `json:"vault_id"`
	Name         *string           `json:"name"`
	AuthType     string            `json:"-"`
	MCPServerURL string            `json:"-"`
	ExpiresAt    *string           `json:"-"`
	Metadata     map[string]string `json:"metadata"`
}

type credentialAuthRead struct {
	Type         string  `json:"type"`
	MCPServerURL string  `json:"mcp_server_url"`
	ExpiresAt    *string `json:"expires_at"`
}

// UnmarshalJSON accepts the documented nested-auth retrieve/create envelope.
func (c *Credential) UnmarshalJSON(data []byte) error {
	type wire struct {
		ID        string              `json:"id"`
		Object    string              `json:"object"`
		CreatedAt int64               `json:"created_at"`
		UpdatedAt int64               `json:"updated_at"`
		VaultID   string              `json:"vault_id"`
		Name      *string             `json:"name"`
		Metadata  map[string]string   `json:"metadata"`
		Auth      *credentialAuthRead `json:"auth"`
	}
	var w wire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	c.ID = w.ID
	c.Object = w.Object
	c.CreatedAt = w.CreatedAt
	c.UpdatedAt = w.UpdatedAt
	c.VaultID = w.VaultID
	c.Name = w.Name
	c.Metadata = w.Metadata
	if w.Auth != nil {
		c.AuthType = w.Auth.Type
		c.MCPServerURL = w.Auth.MCPServerURL
		c.ExpiresAt = w.Auth.ExpiresAt
	}
	return nil
}

// CredentialCreate is credential creation input.
type CredentialCreate struct {
	Name string
	Auth CredentialAuthWrite
}

// CredentialAuthWrite is static bearer or MCP OAuth material.
type CredentialAuthWrite struct {
	Type         string
	MCPServerURL string
	Token        string
	AccessToken  string
	ExpiresAt    Optional[string]
	Refresh      *OAuthRefreshWrite
}

// OAuthRefreshWrite is OAuth refresh configuration.
type OAuthRefreshWrite struct {
	TokenEndpoint     string
	ClientID          string
	RefreshToken      string
	TokenEndpointAuth *TokenEndpointAuthWrite
}

// TokenEndpointAuthWrite is token-endpoint authentication.
type TokenEndpointAuthWrite struct {
	Type         string
	ClientSecret string
}

// CredentialRotate is a rotation request. Omitted optional fields are left
// unchanged; Null clears the corresponding stored value.
type CredentialRotate struct {
	AuthType    string
	Token       Optional[string]
	AccessToken Optional[string]
	ExpiresAt   Optional[string]
	Refresh     Optional[OAuthRefreshWrite]
}

func (w AgentWrite) toMap() map[string]any {
	body := map[string]any{
		"model": w.Model,
	}
	w.Instructions.apply(body, "instructions")
	w.Name.apply(body, "name")
	w.Metadata.apply(body, "metadata")
	w.MultiAgent.apply(body, "multi_agent")
	w.Reasoning.apply(body, "reasoning")
	w.ServiceTier.apply(body, "service_tier")
	w.Text.apply(body, "text")
	w.Tools.apply(body, "tools")
	return body
}

func (w TemplateWrite) toMap() map[string]any {
	body := map[string]any{}
	w.Name.apply(body, "name")
	w.CapabilityDirectories.apply(body, "capability_directories")
	w.Network.apply(body, "network")
	w.Packages.apply(body, "packages")
	if w.SendConfidential {
		w.Files.apply(body, "files")
		w.Skills.apply(body, "skills")
		w.Plugins.apply(body, "plugins")
		w.Env.apply(body, "env")
		w.SetupCommands.apply(body, "setup_commands")
	} else {
		if w.Files.Present && !w.Files.Null {
			body["files"] = publicFiles(w.Files.Value)
		} else {
			w.Files.apply(body, "files")
		}
		if w.Skills.Present && !w.Skills.Null {
			body["skills"] = publicSkills(w.Skills.Value)
		} else {
			w.Skills.apply(body, "skills")
		}
		if w.Plugins.Present && !w.Plugins.Null {
			body["plugins"] = publicPlugins(w.Plugins.Value)
		} else {
			w.Plugins.apply(body, "plugins")
		}
	}
	return body
}

func publicFiles(files []TemplateFile) []TemplateFile {
	out := make([]TemplateFile, len(files))
	for i, f := range files {
		out[i] = TemplateFile{Type: f.Type, Path: f.Path, FileID: f.FileID}
	}
	return out
}

func publicSkills(skills []TemplateSkill) []TemplateSkill {
	out := make([]TemplateSkill, len(skills))
	for i, s := range skills {
		out[i] = s
		if out[i].Source != nil {
			src := *out[i].Source
			src.Data = ""
			out[i].Source = &src
		}
	}
	return out
}

func publicPlugins(plugins []TemplatePlugin) []TemplatePlugin {
	out := make([]TemplatePlugin, len(plugins))
	for i, p := range plugins {
		out[i] = p
		if out[i].Source != nil {
			src := *out[i].Source
			src.Data = ""
			out[i].Source = &src
		}
	}
	return out
}
