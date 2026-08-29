package platforms

import (
	"testing"

	"github.com/langPeanut/langPeanut/pkg/types"
)

func TestReactPlatform_Extract(t *testing.T) {
	p := NewReactPlatform()

	src := []byte(`import React from 'react';
export const Card = () => <div><h2>Checkout Summary</h2><button>Submit Order</button></div>;
`)

	cands, err := p.ExtractCandidates("Card.tsx", src)
	if err != nil {
		t.Fatalf("ExtractCandidates failed: %v", err)
	}

	if len(cands) < 2 {
		t.Fatalf("expected at least 2 candidates, got %d", len(cands))
	}
}

func TestFlutterPlatform_Extract(t *testing.T) {
	p := NewFlutterPlatform()

	src := []byte(`import 'package:flutter/material.dart';
Widget build(BuildContext context) => const Text("Welcome Home");
`)

	cands, err := p.ExtractCandidates("home.dart", src)
	if err != nil {
		t.Fatalf("ExtractCandidates failed: %v", err)
	}

	if len(cands) == 0 {
		t.Fatalf("expected at least 1 candidate, got 0")
	}
}

func TestSwiftPlatform_Extract(t *testing.T) {
	p := NewSwiftPlatform()

	src := []byte(`import SwiftUI
struct View: View {
    var body: some View {
        Text("Settings")
    }
}
`)

	cands, err := p.ExtractCandidates("View.swift", src)
	if err != nil {
		t.Fatalf("ExtractCandidates failed: %v", err)
	}

	if len(cands) == 0 {
		t.Fatalf("expected at least 1 candidate, got 0")
	}
}

func TestKotlinPlatform_Extract(t *testing.T) {
	p := NewAndroidPlatform()

	src := []byte(`package com.app
import androidx.compose.material3.Text
fun Header() {
    Text(text = "Submit Order")
}
`)

	cands, err := p.ExtractCandidates("Header.kt", src)
	if err != nil {
		t.Fatalf("ExtractCandidates failed: %v", err)
	}

	if len(cands) == 0 {
		t.Fatalf("expected at least 1 candidate, got 0")
	}
}

func TestRegistry_AutoDetect(t *testing.T) {
	r := NewRegistry()
	p, _ := r.Get(types.FrameworkReact)
	if p == nil {
		t.Fatalf("expected React platform from registry")
	}
}
