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
