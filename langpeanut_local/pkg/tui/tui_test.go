package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/langPeanut/langPeanut/pkg/agents"
	"github.com/langPeanut/langPeanut/pkg/llm"
	"github.com/langPeanut/langPeanut/pkg/types"
)

func TestTUI_InstantLaunchWithoutScanning(t *testing.T) {
	// Create app targeting Next.js example
	app := NewApp(filepath.Join("..", "..", "examples", "nextjs-app"))

	if app.loading {
		t.Fatalf("Expected app to start with loading=false, got true")
	}
	if len(app.candidates) != 0 {
		t.Fatalf("Expected app to start with 0 candidates before user triggers scan, got %d", len(app.candidates))
	}
	if app.state != ViewMainMenu {
		t.Fatalf("Expected app to start on ViewMainMenu, got %v", app.state)
	}

	// Verify View renders immediately
	rendered := app.View()
	if !strings.Contains(rendered, "Main Menu") {
		t.Fatalf("Expected rendered view to contain main menu dashboard, got:\n%s", rendered)
	}
}

func TestTUI_AsyncScanAndAudit(t *testing.T) {
	app := NewApp(filepath.Join("..", "..", "examples", "nextjs-app"))

	// Trigger Scan (Key "2")
	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m := model.(*Model)

	if !m.loading {
		t.Fatalf("Expected loading=true after triggering scan")
	}
	if cmd == nil {
		t.Fatalf("Expected non-nil tea.Cmd returned for async scan")
	}

	// Verify loading card is rendered in View()
	loadingView := m.View()
	if !strings.Contains(loadingView, "Scanning AST & profiling component elements") {
		t.Fatalf("Expected loading view to render scan loading message, got:\n%s", loadingView)
	}

	// Simulate background completion
	mockCandidates := []types.StringCandidate{
		{ID: "c1", CleanValue: "Welcome", Key: "welcomeTitle", Classification: types.ClassLocalizable, Approved: true},
		{ID: "c2", CleanValue: "Sign In", Key: "signInBtn", Classification: types.ClassLocalizable, Approved: true},
	}
	doneMsg := scanDoneMsg{candidates: mockCandidates}

	model2, _ := m.Update(doneMsg)
	m2 := model2.(*Model)

	if m2.loading {
		t.Fatalf("Expected loading=false after scan completion")
	}
	if m2.state != ViewAudit {
		t.Fatalf("Expected state=ViewAudit, got %v", m2.state)
	}
	if len(m2.candidates) != 2 {
		t.Fatalf("Expected 2 candidates, got %d", len(m2.candidates))
	}

	// Verify Audit view renders candidates cleanly
	auditView := m2.View()
	if !strings.Contains(auditView, "Welcome") || !strings.Contains(auditView, "Sign In") {
		t.Fatalf("Expected audit view to contain candidate strings, got:\n%s", auditView)
	}
}

func TestTUI_1ClickLocalizationAsyncFlow(t *testing.T) {
	app := NewApp(filepath.Join("..", "..", "examples", "nextjs-app"))

	// Trigger 1-Click Localization Wizard (Key "1")
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	m := model.(*Model)

	if m.state != ViewRunWizard {
		t.Fatalf("Expected state=ViewRunWizard, got %v", m.state)
	}
	if m.wizardStep != 0 {
		t.Fatalf("Expected wizardStep=0 (Languages), got %d", m.wizardStep)
	}
	if !strings.Contains(m.View(), "Step 1 of 5") {
		t.Fatalf("Expected wizard step 1 in view, got:\n%s", m.View())
	}

	// Step 1 -> Step 2 (Tone & Style)
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(*Model)
	if m.wizardStep != 1 {
		t.Fatalf("Expected wizardStep=1, got %d", m.wizardStep)
	}
	if !strings.Contains(m.View(), "Step 2 of 5") {
		t.Fatalf("Expected wizard step 2 in view, got:\n%s", m.View())
	}

	// Step 2 -> Step 3 (UI Directive)
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(*Model)
	if m.wizardStep != 2 {
		t.Fatalf("Expected wizardStep=2, got %d", m.wizardStep)
	}
	if !strings.Contains(m.View(), "Step 3 of 5") {
		t.Fatalf("Expected wizard step 3 in view, got:\n%s", m.View())
	}

	// Step 3 -> Step 4 (Safety Mode)
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(*Model)
	if m.wizardStep != 3 {
		t.Fatalf("Expected wizardStep=3, got %d", m.wizardStep)
	}
	if !strings.Contains(m.View(), "Step 4 of 5") {
		t.Fatalf("Expected wizard step 4 in view, got:\n%s", m.View())
	}

	// Step 4 -> Step 5 (Summary Confirmation)
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(*Model)
	if m.wizardStep != 4 {
		t.Fatalf("Expected wizardStep=4, got %d", m.wizardStep)
	}
	if !strings.Contains(m.View(), "Step 5 of 5") {
		t.Fatalf("Expected wizard step 5 summary in view, got:\n%s", m.View())
	}

	// Step 5 -> Launch Pipeline!
	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(*Model)

	if !m.loading {
		t.Fatalf("Expected loading=true after confirming wizard")
	}
	if cmd == nil {
		t.Fatalf("Expected non-nil tea.Cmd returned for background execution")
	}

	loadingView := m.View()
	if !strings.Contains(strings.ToLower(loadingView), "1-click ai localization") {
		t.Fatalf("Expected loading view to render running stage, got:\n%s", loadingView)
	}
}

func TestTUI_ProjectTargetSwitching(t *testing.T) {
	app := NewApp(filepath.Join("..", "..", "examples", "nextjs-app"))

	// Switch to Flutter app
	flutterPath := filepath.Join("..", "..", "examples", "flutter-app")
	app.switchTargetProject(flutterPath)

	if app.loading {
		t.Fatalf("Expected loading=false on project switch")
	}
	if app.platform == nil || app.platform.Name() != types.FrameworkFlutter {
		t.Fatalf("Expected platform to be FrameworkFlutter, got %v", app.platform)
	}
	if !strings.Contains(app.statusMsg, "Switched target to") {
		t.Fatalf("Expected status message confirming switch, got: %s", app.statusMsg)
	}
}

func TestTUI_RealProjectAsyncScanCommandExecution(t *testing.T) {
	app := NewApp(filepath.Join("..", "..", "examples", "nextjs-app"))

	// Trigger scan via key '2'
	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m := model.(*Model)

	if !m.loading {
		t.Fatalf("Expected loading=true")
	}

	// Execute the async tea.Cmd directly in test
	msg := cmd()
	doneMsg, ok := msg.(scanDoneMsg)
	if !ok {
		// When cmd is tea.Batch, check batch messages
		if batchMsgs, isBatch := msg.(tea.BatchMsg); isBatch {
			for _, bCmd := range batchMsgs {
				if bCmd != nil {
					if resMsg := bCmd(); resMsg != nil {
						if sm, isScan := resMsg.(scanDoneMsg); isScan {
							doneMsg = sm
							ok = true
						}
					}
				}
			}
		}
	}

	if ok {
		if doneMsg.err != nil {
			t.Fatalf("Async scan failed: %v", doneMsg.err)
		}
		if len(doneMsg.candidates) == 0 {
			t.Fatalf("Expected candidate strings from nextjs-app, got 0")
		}

		// Feed completion msg back into Update
		model2, _ := m.Update(doneMsg)
		m2 := model2.(*Model)

		if m2.loading {
			t.Fatalf("Expected loading=false after scan done")
		}
		if m2.state != ViewAudit {
			t.Fatalf("Expected state=ViewAudit, got %v", m2.state)
		}
		if len(m2.candidates) == 0 {
			t.Fatalf("Expected candidates to be populated in Model")
		}
	}
}

func TestTUI_InteractiveReviewKeyApprovals(t *testing.T) {
	app := NewApp(filepath.Join("..", "..", "examples", "nextjs-app"))
	app.state = ViewReview
	app.candidates = []types.StringCandidate{
		{ID: "c1", Key: "firstKey", CleanValue: "First", Approved: false},
		{ID: "c2", Key: "secondKey", CleanValue: "Second", Approved: false},
	}
	app.candidateIdx = 0

	// Approve first item ('a')
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m := model.(*Model)
	if !m.candidates[0].Approved {
		t.Fatalf("Expected first candidate to be approved")
	}
	if m.candidateIdx != 1 {
		t.Fatalf("Expected cursor to advance to index 1, got %d", m.candidateIdx)
	}

	// Batch Approve All ('A')
	model2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	m2 := model2.(*Model)
	if !m2.candidates[0].Approved || !m2.candidates[1].Approved {
		t.Fatalf("Expected all candidates to be approved with 'A'")
	}

	// Batch Skip All ('S')
	model3, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
	m3 := model3.(*Model)
	if m3.candidates[0].Approved || m3.candidates[1].Approved {
		t.Fatalf("Expected all candidates to be skipped with 'S'")
	}
}

func TestTUI_OnboardingSetupFlow(t *testing.T) {
	app := NewApp(filepath.Join("..", "..", "examples", "nextjs-app"))

	// Trigger Onboarding (Key "0")
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'0'}})
	m := model.(*Model)

	if m.state != ViewOnboarding {
		t.Fatalf("Expected state=ViewOnboarding, got %v", m.state)
	}
	if m.onboardingStep != 0 {
		t.Fatalf("Expected onboardingStep=0 (AI Engine), got %d", m.onboardingStep)
	}
	if !strings.Contains(m.View(), "Step 1 of 4") {
		t.Fatalf("Expected step 1 in view, got:\n%s", m.View())
	}

	// Step 1 -> Step 2 (API Keys)
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(*Model)
	if m.onboardingStep != 1 {
		t.Fatalf("Expected onboardingStep=1, got %d", m.onboardingStep)
	}
	if !strings.Contains(m.View(), "Step 2 of 4") {
		t.Fatalf("Expected step 2 in view, got:\n%s", m.View())
	}

	// Step 2 -> Step 3 (Defaults)
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(*Model)
	if m.onboardingStep != 2 {
		t.Fatalf("Expected onboardingStep=2, got %d", m.onboardingStep)
	}
	if !strings.Contains(m.View(), "Step 3 of 4") {
		t.Fatalf("Expected step 3 in view, got:\n%s", m.View())
	}

	// Step 3 -> Step 4 (Complete)
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(*Model)
	if m.onboardingStep != 3 {
		t.Fatalf("Expected onboardingStep=3, got %d", m.onboardingStep)
	}
	if !strings.Contains(m.View(), "Step 4 of 4") {
		t.Fatalf("Expected step 4 in view, got:\n%s", m.View())
	}

	// Step 4 -> Complete & Save to Dashboard
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(*Model)
	if m.state != ViewMainMenu {
		t.Fatalf("Expected state=ViewMainMenu after completing onboarding, got %v", m.state)
	}
	if !strings.Contains(strings.ToLower(m.statusMsg), "onboarding complete") {
		t.Fatalf("Expected status message, got: %s", m.statusMsg)
	}
}

func TestTUI_TokenStatsViewAndShortcut(t *testing.T) {
	app := NewApp(filepath.Join("..", "..", "examples", "nextjs-app"))

	// Simulate recording some token usage
	llm.RecordUsage("openai", "gpt-5.4-mini-2026-03-17", 4500, 1800)

	// Press 't' shortcut to open Token Stats
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	m := model.(*Model)

	if m.state != ViewTokenStats {
		t.Fatalf("Expected state=ViewTokenStats after pressing 't', got %v", m.state)
	}

	view := m.View()
	if !strings.Contains(view, "AI Token Consumption & Cost Analytics") {
		t.Fatalf("Expected token stats header in view, got:\n%s", view)
	}
	if !strings.Contains(view, "Input Tokens") || !strings.Contains(view, "Output Tokens") {
		t.Fatalf("Expected KPI cards in view, got:\n%s", view)
	}
	if !strings.Contains(view, "gpt-5.4-mini-2026-03-17") {
		t.Fatalf("Expected model in breakdown table, got:\n%s", view)
	}

	// Press 'r' to reset
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = model.(*Model)
	if !strings.Contains(m.statusMsg, "reset to 0") {
		t.Fatalf("Expected reset confirmation message, got: %s", m.statusMsg)
	}

	// Press 'esc' to return to Main Menu
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = model.(*Model)
	if m.state != ViewMainMenu {
		t.Fatalf("Expected state=ViewMainMenu after pressing esc, got %v", m.state)
	}
}

func TestTUI_WorkflowCompletionSummaryAndDependencyInstall(t *testing.T) {
	app := NewApp(filepath.Join("..", "..", "examples", "nextjs-app"))

	// 1. Simulate completion of 1-click full localization pipeline
	mockResult := &agents.PipelineResult{
		ScannedFilesCount:   5,
		ExtractedCandidates: 12,
		UniqueKeysCount:     12,
		RefactoredFiles:     []string{"src/components/Navbar.tsx", "src/components/Footer.tsx"},
		GeneratedLocales:    []string{"es", "fr", "de", "ja"},
		DependencyStatus: &types.DependencyStatus{
			Framework:       types.FrameworkReact,
			ManifestFile:    "package.json",
			InstalledDeps:   []string{"react-i18next", "i18next"},
			ManifestUpdated: true,
			CommandExecuted: "npm install react-i18next i18next",
			ConfigCreated:   []string{"src/i18n.ts"},
			Success:         true,
		},
	}

	model, _ := app.Update(fullLocDoneMsg{result: mockResult})
	m := model.(*Model)

	// Verify state transitioned to ViewSummary (NOT ViewAudit or raw scan)
	if m.state != ViewSummary {
		t.Fatalf("Expected state=ViewSummary upon workflow completion, got %v", m.state)
	}

	// Verify Summary View renders all key sections cleanly
	view := m.View()
	if !strings.Contains(view, "Execution Summary") {
		t.Fatalf("Expected Execution Summary header in view, got:\n%s", view)
	}
	if !strings.Contains(view, "Framework Dependencies & Manifest Setup") {
		t.Fatalf("Expected Dependencies box in view, got:\n%s", view)
	}
	if !strings.Contains(view, "package.json") || !strings.Contains(view, "npm install react-i18next i18next") {
		t.Fatalf("Expected manifest and install command in view, got:\n%s", view)
	}
	if !strings.Contains(view, "Navbar.tsx") {
		t.Fatalf("Expected refactored file in view, got:\n%s", view)
	}
	if !strings.Contains(view, "Multilingual Catalogs Written") {
		t.Fatalf("Expected multilingual catalogs in view, got:\n%s", view)
	}
	if !strings.Contains(view, "[i] Run Dependency Install") {
		t.Fatalf("Expected [i] action shortcut in view, got:\n%s", view)
	}

	// 2. Press 'i' to trigger dependency install command
	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	m = model.(*Model)

	if !m.loading {
		t.Fatalf("Expected loading=true after pressing 'i' to install dependencies")
	}
	if cmd == nil {
		t.Fatalf("Expected non-nil tea.Cmd for async dependency install")
	}

	// 3. Simulate dependency install completion
	depMsg := installDepsDoneMsg{
		status: &types.DependencyStatus{
			Framework:       types.FrameworkReact,
			ManifestFile:    "package.json",
			CommandExecuted: "npm install",
			Success:         true,
		},
	}
	model, _ = m.Update(depMsg)
	m = model.(*Model)

	if m.loading {
		t.Fatalf("Expected loading=false after dependency install completed")
	}
	if !strings.Contains(m.statusMsg, "npm install") {
		t.Fatalf("Expected status message confirming install, got: %s", m.statusMsg)
	}

	// 4. Press Enter or Esc to return to Main Menu
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(*Model)
	if m.state != ViewMainMenu {
		t.Fatalf("Expected state=ViewMainMenu after pressing enter from summary, got %v", m.state)
	}
}


