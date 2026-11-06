package provider

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// sieveRuleSetModel is the structured form of a Sieve script.
type sieveRuleSetModel struct {
	ID         types.String    `tfsdk:"id"`
	UserHandle types.String    `tfsdk:"user_handle"`
	Password   types.String    `tfsdk:"password"`
	Name       types.String    `tfsdk:"name"`
	Active     types.Bool      `tfsdk:"active"`
	Rules      []sieveRuleSpec `tfsdk:"rule"`
	Content    types.String    `tfsdk:"content"`
}

type sieveRuleSpec struct {
	Description types.String      `tfsdk:"description"`
	Matches     []sieveMatchSpec  `tfsdk:"match"`
	Actions     []sieveActionSpec `tfsdk:"action"`
}

type sieveMatchSpec struct {
	Header types.String `tfsdk:"header"`
	Type   types.String `tfsdk:"type"`
	Value  types.String `tfsdk:"value"`
}

type sieveActionSpec struct {
	Type    types.String   `tfsdk:"type"`
	Target  types.String   `tfsdk:"target"`
	Subject types.String   `tfsdk:"subject"`
	Message types.String   `tfsdk:"message"`
	Days    types.Int64    `tfsdk:"days"`
	Flags   []types.String `tfsdk:"flags"`
}

// generateSieve renders the structured rules into Sieve source. The output
// is deterministic: identical rules always produce identical scripts.
func generateSieve(rules []sieveRuleSpec) (string, error) {
	requires := map[string]bool{}
	var body strings.Builder

	for i, rule := range rules {
		// Sanitize: a description containing newlines could forge Sieve
		// directives, so collapse them into spaces.
		desc := strings.Join(strings.Fields(rule.Description.ValueString()), " ")
		if desc != "" {
			fmt.Fprintf(&body, "# %s\n", desc)
		}

		tests := make([]string, 0, len(rule.Matches))
		for _, m := range rule.Matches {
			test, err := renderMatch(m)
			if err != nil {
				return "", err
			}
			tests = append(tests, test)
		}

		actions, err := renderActions(rule.Actions, requires)
		if err != nil {
			return "", err
		}
		if actions == "" {
			return "", fmt.Errorf("rule %d has no actions", i)
		}

		switch {
		case len(tests) == 0:
			// Unconditional rule: actions run for every message.
			body.WriteString(actions)
		case len(tests) == 1:
			fmt.Fprintf(&body, "if %s {\n%s}\n", tests[0], indent(actions))
		default:
			fmt.Fprintf(&body, "if allof (%s) {\n%s}\n", strings.Join(tests, ", "), indent(actions))
		}
		body.WriteString("\n")
	}

	var out strings.Builder
	if len(requires) > 0 {
		names := make([]string, 0, len(requires))
		for name := range requires {
			names = append(names, `"`+name+`"`)
		}
		// Deterministic order.
		sortStrings(names)
		fmt.Fprintf(&out, "require [%s];\n\n", strings.Join(names, ", "))
	}
	out.WriteString(strings.TrimRight(body.String(), "\n"))
	out.WriteString("\n")
	return out.String(), nil
}

func renderMatch(m sieveMatchSpec) (string, error) {
	header := m.Header.ValueString()
	value := sieveQuoted(m.Value.ValueString())
	switch m.Type.ValueString() {
	case "is":
		return fmt.Sprintf(`header :is %q %s`, header, value), nil
	case "contains":
		return fmt.Sprintf(`header :contains %q %s`, header, value), nil
	case "matches":
		return fmt.Sprintf(`header :matches %q %s`, header, value), nil
	default:
		return "", fmt.Errorf("unsupported match type %q (want is, contains, or matches)", m.Type.ValueString())
	}
}

func renderActions(actions []sieveActionSpec, requires map[string]bool) (string, error) {
	var body strings.Builder
	for _, a := range actions {
		target := sieveQuoted(a.Target.ValueString())
		switch a.Type.ValueString() {
		case "fileinto":
			requires["fileinto"] = true
			fmt.Fprintf(&body, "  fileinto %s;\n", target)
		case "redirect":
			fmt.Fprintf(&body, "  redirect %s;\n", target)
		case "discard":
			body.WriteString("  discard;\n")
		case "stop":
			body.WriteString("  stop;\n")
		case "addflag":
			requires["imap4flags"] = true
			flags := make([]string, 0, len(a.Flags))
			for _, f := range a.Flags {
				flags = append(flags, sieveQuoted(f.ValueString()))
			}
			if len(flags) == 0 {
				return "", fmt.Errorf("addflag action requires at least one flag")
			}
			fmt.Fprintf(&body, "  addflag [%s];\n", strings.Join(flags, ", "))
		case "vacation":
			requires["vacation"] = true
			var opts strings.Builder
			if !a.Days.IsNull() {
				fmt.Fprintf(&opts, " :days %d", a.Days.ValueInt64())
			}
			if a.Subject.ValueString() != "" {
				fmt.Fprintf(&opts, " :subject %s", sieveQuoted(a.Subject.ValueString()))
			}
			fmt.Fprintf(&body, "  vacation%s %s;\n", opts.String(), sieveQuoted(a.Message.ValueString()))
		default:
			return "", fmt.Errorf("unsupported action type %q", a.Type.ValueString())
		}
	}
	return body.String(), nil
}

func indent(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		lines[i] = "  " + l
	}
	return strings.Join(lines, "\n") + "\n"
}

func sieveQuoted(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

func sortStrings(s []string) {
	sort.Strings(s)
}
