// Copyright (c) MemoryStore 2026
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"log"
	"net/http"
	"os"

	"github.com/MemoryStore/terraform-provider-openaiagents/internal/testfake"
)

func main() {
	addr := ":8080"
	if v := os.Getenv("FAKE_AGENTS_API_ADDR"); v != "" {
		addr = v
	}
	log.Printf("fake Agents API listening on %s; set provider base_url to http://127.0.0.1%s/v1", addr, addr)
	log.Fatal(http.ListenAndServe(addr, testfake.New()))
}
