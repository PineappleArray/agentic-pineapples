package rulesengine

import (
	"fmt"
	"os"

	"github.com/google/cel-go/cel"
	"gopkg.in/yaml.v3"

	"github.com/PineappleArray/agentic-pineapples/internal/types"
)

const DefaultPath = "./rules.yaml"

// FlagDef is a deterministic keyword/phrase lexicon entry, matched by the
// enrichment layer before extraction to populate event.text_flags.
type FlagDef struct {
	Match []string `yaml:"match"`
}

// CompiledRule pairs a rule with its CEL program, compiled once at load time
// so evaluation never re-parses the condition source.
type CompiledRule struct {
	types.Rule
	Program cel.Program
}

// Ruleset is the fully loaded, CEL-compiled policy: everything the engine
// needs to evaluate an event without touching rules.yaml again.
type Ruleset struct {
	Version string
	Flags   map[string]FlagDef
	Rules   []CompiledRule
}

// ruleFile mirrors the on-disk shape of rules.yaml.
type ruleFile struct {
	Version string             `yaml:"version"`
	Flags   map[string]FlagDef `yaml:"flags"`
	Rules   []types.Rule       `yaml:"rules"`
}

// celEnv declares the "event" variable rule conditions are evaluated
// against. It's left dynamically typed here; resolver.go is responsible for
// building the matching activation from a types.Event at evaluation time.
func celEnv() (*cel.Env, error) {
	return cel.NewEnv(
		cel.Variable("event", cel.MapType(cel.StringType, cel.DynType)),
	)
}

// Load reads rules.yaml at path and compiles every rule's CEL condition.
func Load(path string) (*Ruleset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading ruleset %s: %w", path, err)
	}

	var rf ruleFile
	if err := yaml.Unmarshal(data, &rf); err != nil {
		return nil, fmt.Errorf("parsing ruleset %s: %w", path, err)
	}

	env, err := celEnv()
	if err != nil {
		return nil, fmt.Errorf("building CEL environment: %w", err)
	}

	rules := make([]CompiledRule, 0, len(rf.Rules))
	for _, rule := range rf.Rules {
		ast, iss := env.Compile(rule.Condition)
		if iss != nil && iss.Err() != nil {
			return nil, fmt.Errorf("compiling rule %q: %w", rule.ID, iss.Err())
		}
		prg, err := env.Program(ast)
		if err != nil {
			return nil, fmt.Errorf("building program for rule %q: %w", rule.ID, err)
		}
		rules = append(rules, CompiledRule{Rule: rule, Program: prg})
	}

	rs := &Ruleset{
		Version: rf.Version,
		Flags:   rf.Flags,
		Rules:   rules,
	}
	if e := rs.validate_type(); e != nil {
		return nil, e
	} else {
		return rs, nil
	}
}
