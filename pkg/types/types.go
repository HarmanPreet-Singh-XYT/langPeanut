package types

import "time"

// Framework represents the supported target framework
type Framework string

const (
	FrameworkReact     Framework = "react"
	FrameworkNextJS    Framework = "nextjs"
	FrameworkReactNative Framework = "react_native"
	FrameworkFlutter   Framework = "flutter"
	FrameworkSwiftUI   Framework = "swiftui"
	FrameworkAndroid   Framework = "android"
	FrameworkVue       Framework = "vue"
	FrameworkAngular   Framework = "angular"
	FrameworkPython    Framework = "python"
	FrameworkGo        Framework = "go"
	FrameworkGeneric   Framework = "generic"
)

// Classification represents whether a string is UI, Code, or Uncertain
type Classification string

const (
	ClassLocalizable Classification = "LOCALIZABLE"
	ClassSkip        Classification = "SKIP"
	ClassUncertain   Classification = "UNCERTAIN"
)

// StringCandidate represents an extracted candidate string literal
type StringCandidate struct {
	ID             string         `json:"id"`
	FilePath       string         `json:"file_path"`
	StartByte      int            `json:"start_byte"`
	EndByte        int            `json:"end_byte"`
	StartLine      int            `json:"start_line"`
	StartCol       int            `json:"start_col"`
	EndLine        int            `json:"end_line"`
	EndCol         int            `json:"end_col"`
	RawValue       string         `json:"raw_value"`
	CleanValue     string         `json:"clean_value"`
	Key            string         `json:"key"`
	ParentNodeType string         `json:"parent_node_type"`
	ContextHint    string         `json:"context_hint"`
	SiblingStrings []string       `json:"sibling_strings,omitempty"`
	Classification Classification `json:"classification"`
	Confidence     float64        `json:"confidence"`
	Explanation    string         `json:"explanation,omitempty"`
	Variables      []string       `json:"variables,omitempty"`
	IsPlural       bool           `json:"is_plural"`
	PluralForms    map[string]string `json:"plural_forms,omitempty"`
	Approved       bool           `json:"approved"`
	IsConstContext bool           `json:"is_const_context,omitempty"`
	// ConstByteRange holds the exact [start,end) byte offsets of the `const`
	// keyword token that must be stripped once this candidate's string literal
	// is replaced with a non-constant lookup, when known precisely (e.g. from
	// AST-based extraction). Nil means no precise range was recorded.
	ConstByteRange *[2]int `json:"const_byte_range,omitempty"`
}

// ByteRangePatch represents a surgical code replacement in a file
type ByteRangePatch struct {
	FilePath        string `json:"file_path"`
	StartByte       int    `json:"start_byte"`
	EndByte         int    `json:"end_byte"`
	ReplacementText string `json:"replacement_text"`
	Description     string `json:"description"`
}

// FileRefactorPlan represents the complete refactoring actions for a single source file
type FileRefactorPlan struct {
	FilePath         string           `json:"file_path"`
	OriginalContent  string           `json:"original_content"`
	Patches          []ByteRangePatch `json:"patches"`
	RequiredImports  []string         `json:"required_imports"`
	RequiredHooks    []string         `json:"required_hooks"`
	RemoveConstBytes [][2]int         `json:"remove_const_bytes,omitempty"`
	RefactoredContent string          `json:"refactored_content,omitempty"`
}

// LocaleData represents key-value translations for a locale
type LocaleData struct {
	LocaleCode string            `json:"locale_code"`
	Format     string            `json:"format"` // arb, json_i18next, strings_xml, xcstrings
	Entries    map[string]string `json:"entries"`
	Metadata   map[string]any    `json:"metadata,omitempty"` // for ARB @key attributes
}

// VerificationTier represents the tier level in the 4-Tier Critic
type VerificationTier int

const (
	Tier1SyntaxAST VerificationTier = 1
	Tier2ICUTokens VerificationTier = 2
	Tier3UIExpansion VerificationTier = 3
	Tier4LocaleParity VerificationTier = 4
)

// Diagnostic represents an error or warning emitted by the Verifier Critic
type Diagnostic struct {
	Tier        VerificationTier `json:"tier"`
	Severity    string           `json:"severity"` // "ERROR", "WARNING", "INFO"
	Key         string           `json:"key,omitempty"`
	Locale      string           `json:"locale,omitempty"`
	Message     string           `json:"message"`
	Expected    string           `json:"expected,omitempty"`
	Actual      string           `json:"actual,omitempty"`
	CanAutoFix  bool             `json:"can_auto_fix"`
	AutoFixHint string           `json:"auto_fix_hint,omitempty"`
}

// CompilerDiagnostic represents a compiler, linter, or AST syntax error for a specific file
type CompilerDiagnostic struct {
	FilePath string `json:"file_path"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Message  string `json:"message"`
	Source   string `json:"source"` // "tsc", "flutter analyze", "swift", "ast", "kotlinc", "go vet"
	Severity string `json:"severity"` // "ERROR", "WARNING"
}

// CodeRepairResult represents the outcome of an automated code self-healing attempt
type CodeRepairResult struct {
	FilePath        string               `json:"file_path"`
	OriginalErrors  []CompilerDiagnostic `json:"original_errors"`
	Repaired        bool                 `json:"repaired"`
	RemainingErrors []CompilerDiagnostic `json:"remaining_errors,omitempty"`
	Attempts        int                  `json:"attempts"`
	Explanation     string               `json:"explanation,omitempty"`
}

// DirectiveResult represents the outcome of the Post-Localization App Integration Agent
type DirectiveResult struct {
	Directive      string   `json:"directive"`
	Success        bool     `json:"success"`
	CreatedFiles   []string `json:"created_files,omitempty"`
	PatchedFiles   []string `json:"patched_files,omitempty"`
	Explanation    string   `json:"explanation,omitempty"`
	CompilerPassed bool     `json:"compiler_passed"`
	Attempts       int      `json:"attempts"`
}

// DependencyStatus records the outcome of localization framework dependency checks and installation
type DependencyStatus struct {
	Framework       Framework `json:"framework"`
	ManifestFile    string    `json:"manifest_file,omitempty"`
	MissingDeps     []string  `json:"missing_dependencies,omitempty"`
	InstalledDeps   []string  `json:"installed_dependencies,omitempty"`
	ManifestUpdated bool      `json:"manifest_updated"`
	CommandExecuted string    `json:"command_executed,omitempty"`
	CommandOutput   string    `json:"command_output,omitempty"`
	ConfigCreated   []string  `json:"config_created,omitempty"`
	Success         bool      `json:"success"`
	Message         string    `json:"message"`
}

// VerificationReport contains all diagnostics from the 4-tier critic
type VerificationReport struct {
	Passed      bool         `json:"passed"`
	Diagnostics []Diagnostic `json:"diagnostics"`
	ErrorCount  int          `json:"error_count"`
	WarnCount   int          `json:"warn_count"`
}

// TrajectoryStep captures an individual step in the agent workflow for Hackathon Deliverable 04
type TrajectoryStep struct {
	StepIndex    int            `json:"step_index"`
	Timestamp    time.Time      `json:"timestamp"`
	AgentName    string         `json:"agent_name"`
	Action       string         `json:"action"`
	Thought      string         `json:"thought,omitempty"`
	ToolCall     string         `json:"tool_call,omitempty"`
	ToolInput    any            `json:"tool_input,omitempty"`
	ToolOutput   any            `json:"tool_output,omitempty"`
	CriticFeedback string       `json:"critic_feedback,omitempty"`
	RetryCount   int            `json:"retry_count,omitempty"`
	PassedCheck  bool           `json:"passed_check"`
}

// SessionState tracks the persistent session state for checkpoint and resume
type SessionState struct {
	SessionID        string                     `json:"session_id"`
	Stage            string                     `json:"stage"` // "init", "scanned", "classified", "refactored", "translated", "verified"
	CreatedAt        time.Time                  `json:"created_at"`
	UpdatedAt        time.Time                  `json:"updated_at"`
	ProjectRoot      string                     `json:"project_root"`
	DetectedFramework Framework                 `json:"detected_framework"`
	SourceLocale     string                     `json:"source_locale"`
	TargetLocales    []string                   `json:"target_locales"`
	Candidates       map[string]StringCandidate `json:"candidates"`
	RefactorPlans    map[string]FileRefactorPlan `json:"refactor_plans"`
	LocaleFiles      map[string]LocaleData      `json:"locale_files"`
	Trajectory       []TrajectoryStep           `json:"trajectory"`
	Checkpoints      []string                   `json:"checkpoints"`
}

// LanguageMeta describes a supported language
type LanguageMeta struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// GlobalLanguages contains the complete 100+ language directory
var GlobalLanguages = []LanguageMeta{
	// Americas & Western
	{"es", "Spanish (Español)"},
	{"es-MX", "Spanish - Mexico (Español de México)"},
	{"es-AR", "Spanish - Argentina (Español de Argentina)"},
	{"fr", "French (Français)"},
	{"fr-CA", "French - Canada (Français canadien)"},
	{"de", "German (Deutsch)"},
	{"de-AT", "German - Austria (Österreichisches Deutsch)"},
	{"de-CH", "German - Switzerland (Schweizer Hochdeutsch)"},
	{"it", "Italian (Italiano)"},
	{"pt", "Portuguese - Portugal (Português)"},
	{"pt-BR", "Portuguese - Brazil (Português do Brasil)"},
	{"nl", "Dutch (Nederlands)"},
	{"nl-BE", "Flemish - Belgium (Vlaams)"},
	{"en-GB", "English - UK (British English)"},
	{"en-AU", "English - Australia (Australian English)"},
	{"en-CA", "English - Canada (Canadian English)"},

	// East Asia & Southeast Asia
	{"ja", "Japanese (日本語)"},
	{"zh-CN", "Chinese - Simplified (简体中文)"},
	{"zh-TW", "Chinese - Traditional (繁體中文)"},
	{"zh-HK", "Chinese - Hong Kong (香港粵語)"},
	{"ko", "Korean (한국어)"},
	{"vi", "Vietnamese (Tiếng Việt)"},
	{"th", "Thai (ไทย)"},
	{"id", "Indonesian (Bahasa Indonesia)"},
	{"ms", "Malay (Bahasa Melayu)"},
	{"fil", "Filipino / Tagalog"},
	{"my", "Burmese (မြန်မာစာ)"},
	{"km", "Khmer (ភាសាខ្មែਰ)"},
	{"lo", "Lao (ພາສາລາວ)"},
	{"mn", "Mongolian (Монгол хэл)"},

	// South Asia (Indic & Dravidian)
	{"hi", "Hindi (हिन्दी)"},
	{"pa", "Punjabi (ਪੰਜਾਬੀ)"},
	{"bn", "Bengali (বাংলা)"},
	{"ur", "Urdu (اردو)"},
	{"ta", "Tamil (தமிழ்)"},
	{"te", "Telugu (తెలుగు)"},
	{"mr", "Marathi (मराठी)"},
	{"gu", "Gujarati (ગુજરાતી)"},
	{"kn", "Kannada (ಕನ್ನಡ)"},
	{"ml", "Malayalam (മലയാളം)"},
	{"or", "Odia (ଓଡ଼ିਆ)"},
	{"as", "Assamese (অসমীয়া)"},
	{"ne", "Nepali (नेपाली)"},
	{"si", "Sinhala (සිංහල)"},
	{"sd", "Sindhi (سنڌي)"},
	{"sa", "Sanskrit (संस्कृतम्)"},

	// Middle East & Central Asia
	{"ar", "Arabic (العربية)"},
	{"ar-SA", "Arabic - Saudi Arabia (العربية السعودية)"},
	{"ar-EG", "Arabic - Egypt (العربية المصرية)"},
	{"he", "Hebrew (עברית)"},
	{"fa", "Persian / Farsi (فارسی)"},
	{"tr", "Turkish (Türkçe)"},
	{"az", "Azerbaijani (Azərbaycan dili)"},
	{"kk", "Kazakh (Қазақ тілі)"},
	{"uz", "Uzbek (Oʻzbekcha)"},
	{"ky", "Kyrgyz (Кыргызча)"},
	{"tg", "Tajik (Тоҷикӣ)"},
	{"tk", "Turkmen (Türkmençe)"},
	{"ps", "Pashto (پښتو)"},
	{"ku", "Kurdish (Kurdî)"},
	{"hy", "Armenian (Հայերեն)"},
	{"ka", "Georgian (ქართული)"},

	// Northern, Western & Southern Europe
	{"sv", "Swedish (Svenska)"},
	{"da", "Danish (Dansk)"},
	{"fi", "Finnish (Suomi)"},
	{"no", "Norwegian Bokmål (Norsk Bokmål)"},
	{"nn", "Norwegian Nynorsk (Nynorsk)"},
	{"is", "Icelandic (Íslenska)"},
	{"ga", "Irish (Gaeilge)"},
	{"cy", "Welsh (Cymraeg)"},
	{"gd", "Scottish Gaelic (Gàidhlig)"},
	{"eu", "Basque (Euskara)"},
	{"ca", "Catalan (Català)"},
	{"gl", "Galician (Galego)"},
	{"el", "Greek (Ελληνικά)"},
	{"mt", "Maltese (Malti)"},
	{"lb", "Luxembourgish (Lëtzebuergesch)"},
	{"fo", "Faroese (Føroyskt)"},

	// Eastern Europe & Slavic
	{"ru", "Russian (Русский)"},
	{"uk", "Ukrainian (Українська)"},
	{"pl", "Polish (Polski)"},
	{"cs", "Czech (Čeština)"},
	{"sk", "Slovak (Slovenčina)"},
	{"sl", "Slovenian (Slovenščina)"},
	{"hr", "Croatian (Hrvatski)"},
	{"sr", "Serbian (Српски)"},
	{"bs", "Bosnian (Bosanski)"},
	{"bg", "Bulgarian (Български)"},
	{"ro", "Romanian (Română)"},
	{"hu", "Hungarian (Magyar)"},
	{"lt", "Lithuanian (Lietuvių)"},
	{"lv", "Latvian (Latviešu)"},
	{"et", "Estonian (Eesti)"},
	{"sq", "Albanian (Shqip)"},
	{"mk", "Macedonian (Македонски)"},
	{"be", "Belarusian (Беларуская)"},

	// Africa & Indigenous
	{"sw", "Swahili (Kiswahili)"},
	{"am", "Amharic (አማርኛ)"},
	{"ha", "Hausa (Harshen Hausa)"},
	{"yo", "Yoruba (Èdè Yorùbá)"},
	{"ig", "Igbo (Asụsụ Igbo)"},
	{"zu", "Zulu (isiZulu)"},
	{"xh", "Xhosa (isiXhosa)"},
	{"af", "Afrikaans"},
	{"so", "Somali (Soomaaliga)"},
	{"om", "Oromo (Afaan Oromoo)"},
	{"ti", "Tigrinya (ትግርኛ)"},
	{"mg", "Malagasy (Fiteny Malagasy)"},
	{"rw", "Kinyarwanda"},
	{"ny", "Chichewa (ChiCheŵa)"},
	{"st", "Sesotho (Sesotho)"},
	{"sn", "Shona (chiShona)"},
	{"qu", "Quechua (Runasimi)"},
	{"gn", "Guaraní (Avañe'ẽ)"},
	{"mi", "Maori (Te Reo Māori)"},
	{"haw", "Hawaiian (ʻŌlelo Hawaiʻi)"},
	{"sm", "Samoan (Gagana Samoa)"},
	{"to", "Tongan (Lea Faka-Tonga)"},
	{"eo", "Esperanto"},
	{"la", "Latin (Lingua Latina)"},
}

// RawExamples contains raw baseline source code before localization
var RawExamples = map[string]string{
	"nextjs": `import React from 'react';

export interface NavbarProps {
  user?: { name: string; email: string };
  cartCount: number;
  onOpenCart: () => void;
}

export const Navbar: React.FC<NavbarProps> = ({ user, cartCount, onOpenCart }) => {
  return (
    <header className="navbar-container">
      <div className="brand-logo">
        <h1>FlightPeanut Store</h1>
      </div>
      <nav className="nav-links">
        <a href="/flights">Flights</a>
        <a href="/hotels">Hotels</a>
        <a href="/deals">Deals</a>
      </nav>
      <div className="nav-actions">
        <button onClick={onOpenCart} title="View your shopping cart">
          Cart ({cartCount})
        </button>
        {user ? (
          <div className="user-profile">
            <span>Welcome back, {user.name}!</span>
            <button onClick={() => console.log('LOGOUT_TRIGGERED')}>Sign Out</button>
          </div>
        ) : (
          <button onClick={() => console.log('LOGIN_TRIGGERED')}>Sign In</button>
        )}
      </div>
    </header>
  );
};`,

	"flutter": `import 'package:flutter/material.dart';

void main() {
  runApp(const MyApp());
}

class MyApp extends StatelessWidget {
  const MyApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'FlightPeanut Mobile',
      home: const HomeScreen(),
    );
  }
}

class HomeScreen extends StatelessWidget {
  const HomeScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text("Dashboard"),
      ),
      body: Center(
        child: Column(
          children: const [
            Text("Welcome back, {name}!"),
            Tooltip(message: "View settings"),
          ],
        ),
      ),
    );
  }
}`,

	"swiftui": `import SwiftUI

public struct ContentView: View {
    @State private var notificationsEnabled = true

    public init() {}

    public var body: some View {
        NavigationStack {
            VStack(spacing: 20) {
                Text("Welcome back, {name}!")
                    .font(.headline)
                
                Button("Submit Order") {
                    print("Order clicked")
                }
                .buttonStyle(.borderedProminent)
            }
            .navigationTitle("Dashboard")
        }
    }
}`,

	"android": `package com.example.app

import androidx.compose.material3.Button
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable

@Composable
fun OrderScreen() {
    Text(text = "Welcome back, {name}!")
    Button(onClick = { /* process order */ }) {
        Text(text = "Submit Order")
    }
}`,
}
