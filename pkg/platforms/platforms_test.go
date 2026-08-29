package platforms

import (
	"testing"

	"github.com/langPeanut/langPeanut/pkg/types"
)

func TestReactPlatform_Extract(t *testing.T) {
	_ = types.FrameworkReact
	p := NewReactPlatform()

	src := []byte(`import React from 'react';
export const Card = () => (
	<div>
		<h2>Checkout Summary</h2>
		<button>Submit Order</button>
		<div>Clone & install dependencies</div>
		<div>Mobile (iOS & Android)</div>
		<p>Found something that isn&apos;t working right?</p>
	</div>
);
`)

	cands, err := p.ExtractCandidates("Card.tsx", src)
	if err != nil {
		t.Fatalf("ExtractCandidates failed: %v", err)
	}

	for i, c := range cands {
		t.Logf("[%d] Candidate: %q (Raw: %q)", i+1, c.CleanValue, c.RawValue)
	}

	if len(cands) != 5 {
		t.Fatalf("expected 5 candidates, got %d", len(cands))
	}

	if cands[2].CleanValue != "Clone & install dependencies" {
		t.Errorf("expected candidate 3 to be %q, got %q", "Clone & install dependencies", cands[2].CleanValue)
	}

	if cands[3].CleanValue != "Mobile (iOS & Android)" {
		t.Errorf("expected candidate 4 to be %q, got %q", "Mobile (iOS & Android)", cands[3].CleanValue)
	}

	if cands[4].CleanValue != "Found something that isn't working right?" {
		t.Errorf("expected candidate 5 to be %q, got %q", "Found something that isn't working right?", cands[4].CleanValue)
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

func TestFlutterPlatform_Case04(t *testing.T) {
	p := NewFlutterPlatform()
	src := []byte(`import 'package:flutter/material.dart';

class HomeScreen extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text("Welcome Home"),
      ),
      body: Center(
        child: Column(
          children: const [
            Text("Your Dashboard"),
            Tooltip(message: "View details"),
          ],
        ),
      ),
    );
  }
}
`)

	cands, err := p.ExtractCandidates("04_flutter_const_tree.dart", src)
	if err != nil {
		t.Fatalf("ExtractCandidates failed: %v", err)
	}
	t.Logf("Found %d candidates", len(cands))
	for _, c := range cands {
		t.Logf("Candidate: %s -> %s", c.Key, c.CleanValue)
	}

	plan, err := p.GenerateRefactorPlan("04_flutter_const_tree.dart", src, cands)
	if err != nil {
		t.Fatalf("GenerateRefactorPlan failed: %v", err)
	}

	if !ParsesCleanly("04_flutter_const_tree.dart", []byte(plan.RefactoredContent)) {
		t.Logf("Refactored code:\n%s", plan.RefactoredContent)
		t.Errorf("Refactored code failed ParsesCleanly")
	}
}

func TestFlutterPlatform_Case05(t *testing.T) {
	p := NewFlutterPlatform()
	src := []byte(`{
  "@@locale": "en",
  "welcomeUser": "Welcome back, {name}!",
  "@welcomeUser": {
    "description": "Greeting message",
    "placeholders": {
      "name": { "type": "String" }
    }
  },
  "itemCount": "You have {count} items in your cart",
  "@itemCount": {
    "placeholders": {
      "count": { "type": "int" }
    }
  }
}
`)

	cands, err := p.ExtractCandidates("05_flutter_complex_icu.arb", src)
	if err != nil {
		t.Fatalf("ExtractCandidates failed: %v", err)
	}
	t.Logf("Found %d ARB candidates", len(cands))
	plan, err := p.GenerateRefactorPlan("05_flutter_complex_icu.arb", src, cands)
	if err != nil {
		t.Fatalf("GenerateRefactorPlan failed: %v", err)
	}
	if !ParsesCleanly("05_flutter_complex_icu.arb", []byte(plan.RefactoredContent)) {
		t.Errorf("Refactored ARB failed ParsesCleanly")
	}
}
