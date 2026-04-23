package interactive

import (
	"os"
	"reflect"
	"testing"

	"github.com/srcodee/comot/internal/model"
)

func TestCompleteWithPrompterSingleURLFlow(t *testing.T) {
	cfg := model.Config{}
	builtins := []model.PatternDefinition{
		{Name: "email", Regex: `[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`},
	}

	result, err := CompleteWithPrompter(cfg, builtins, scriptedPrompter{
		answers: ScriptedAnswers{
			TargetURL:    "https://example.com",
			BuiltinNames: []string{"email"},
			Format:       "pattern,pattern_name,resource_url,matched_value",
			OutputType:   model.OutputPlain,
		},
	})
	if err != nil {
		t.Fatalf("CompleteWithPrompter returned error: %v", err)
	}

	if result.URL != "https://example.com" {
		t.Fatalf("unexpected URL: %q", result.URL)
	}
	if got, want := len(result.PatternDefs), 1; got != want {
		t.Fatalf("unexpected pattern defs len: got %d want %d", got, want)
	}
	if result.PatternDefs[0].Name != "email" {
		t.Fatalf("unexpected selected pattern: %q", result.PatternDefs[0].Name)
	}
	if !reflect.DeepEqual(result.Format, []string{"pattern", "pattern_name", "resource_url", "matched_value"}) {
		t.Fatalf("unexpected format: %#v", result.Format)
	}
}

func TestCompleteWithPrompterListFlowDoesNotRequireURL(t *testing.T) {
	cfg := model.Config{ListPath: "/tmp/targets.txt"}
	builtins := []model.PatternDefinition{
		{Name: "URL", Regex: `https?://[^\s"'<>]+`},
	}

	result, err := CompleteWithPrompter(cfg, builtins, scriptedPrompter{
		answers: ScriptedAnswers{
			BuiltinNames: []string{"URL"},
			OutputType:   model.OutputPlain,
		},
	})
	if err != nil {
		t.Fatalf("CompleteWithPrompter returned error: %v", err)
	}
	if result.URL != "" {
		t.Fatalf("expected URL to stay empty for list flow, got %q", result.URL)
	}
	if got, want := len(result.PatternDefs), 1; got != want {
		t.Fatalf("unexpected pattern defs len: got %d want %d", got, want)
	}
}

func TestCompleteWithPrompterHistoryFlowDoesNotRequireURL(t *testing.T) {
	root := t.TempDir()
	childA := root + "/scan-a"
	childB := root + "/scan-b"
	if err := os.MkdirAll(childA, 0o755); err != nil {
		t.Fatalf("mkdir childA: %v", err)
	}
	if err := os.MkdirAll(childB, 0o755); err != nil {
		t.Fatalf("mkdir childB: %v", err)
	}
	if err := os.WriteFile(childA+"/index.txt", []byte(""), 0o644); err != nil {
		t.Fatalf("write childA index: %v", err)
	}
	if err := os.WriteFile(childB+"/index.txt", []byte(""), 0o644); err != nil {
		t.Fatalf("write childB index: %v", err)
	}

	cfg := model.Config{HistoryDir: root}
	builtins := []model.PatternDefinition{
		{Name: "URL", Regex: `https?://[^\s"'<>]+`},
	}

	result, err := CompleteWithPrompter(cfg, builtins, scriptedPrompter{
		answers: ScriptedAnswers{
			HistoryDirs:  []string{"[all]"},
			BuiltinNames: []string{"URL"},
			OutputType:   model.OutputPlain,
		},
	})
	if err != nil {
		t.Fatalf("CompleteWithPrompter returned error: %v", err)
	}
	if result.URL != "" {
		t.Fatalf("expected URL to stay empty for history flow, got %q", result.URL)
	}
	if got, want := len(result.HistoryDirs), 2; got != want {
		t.Fatalf("unexpected history dirs len: got %d want %d", got, want)
	}
}

func TestNewPrompterFromEnv(t *testing.T) {
	t.Setenv(scriptedEnv, `{"builtin_names":["email"],"output_type":"plain"}`)
	prompter, err := newPrompterFromEnv()
	if err != nil {
		t.Fatalf("newPrompterFromEnv returned error: %v", err)
	}
	if _, ok := prompter.(scriptedPrompter); !ok {
		t.Fatalf("expected scriptedPrompter, got %T", prompter)
	}
}

func TestNewPrompterFromEnvInvalidJSON(t *testing.T) {
	_ = os.Setenv(scriptedEnv, `{`)
	defer os.Unsetenv(scriptedEnv)

	if _, err := newPrompterFromEnv(); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
