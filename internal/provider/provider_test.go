// Copyright (c) MemoryStore 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	"github.com/MemoryStore/terraform-provider-openaiagents/internal/testfake"
)

var testFake *testfake.Server

func TestMain(m *testing.M) {
	if os.Getenv("OPENAIAGENTS_ACC_LIVE") != "1" {
		_ = os.Unsetenv("OPENAI_API_KEY")
		if os.Getenv("OPENAI_ADMIN_KEY") == "" {
			_ = os.Setenv("OPENAI_ADMIN_KEY", "ADMIN_CANARY_DO_NOT_SEND")
		}
	}
	s, err := testfake.Listen()
	if err != nil {
		fmt.Fprintf(os.Stderr, "fake API: %v\n", err)
		os.Exit(1)
	}
	testFake = s
	code := m.Run()
	_ = s.Close()
	os.Exit(code)
}

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"openaiagents": providerserver.NewProtocol6WithError(newTestProvider("test")()),
}

func testProviderConfig(baseURL string) string {
	if baseURL == "" && testFake != nil {
		baseURL = testFake.URL()
	}
	return fmt.Sprintf(`
provider "openaiagents" {
  api_key  = "sk-test"
  base_url = %q
}
`, baseURL)
}

func testConfig() string {
	return testProviderConfig(testFake.URL())
}

// testProviderConfigWithoutKey declares the provider with no api_key so the
// resolved credential comes from the environment, or is absent entirely.
func testProviderConfigWithoutKey() string {
	return fmt.Sprintf(`
provider "openaiagents" {
  base_url = %q
}
`, testFake.URL())
}
