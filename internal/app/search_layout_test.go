package app

import (
	"strings"
	"testing"
)

// The phone search results were reported as "not the same width as display,
// shifted to the left, cannot see the whole result". Two causes, chain of
// reasoning below, and both were cascade facts that no Go assertion saw:
//
//  1. The phone overrides were written BEFORE the desktop
//     .header-search-results rule. A media query adds no specificity, so the
//     desktop rule won and the panel kept `left: 50%; transform:
//     translateX(-50%)` — half its width off the left of the field.
//  2. The panel was anchored to the search field, which on a phone is only
//     the space the header can spare (setup and the account buttons take the
//     rest): a full-width panel cannot be measured from a 178px column.
//
// The tests below read the stylesheet the way the browser resolves it: the
// rule that matters is the LAST one with a given selector, so order in the
// file is the assertion.

// ruleIndex returns the position of every rule whose own selector text is
// exactly selector — not a longer selector that merely ends with it.
func ruleIndex(css, selector string) []int {
	var found []int
	for from := 0; ; {
		rel := strings.Index(css[from:], selector)
		if rel < 0 {
			return found
		}
		at := from + rel
		from = at + len(selector)
		// The character after the selector must be the opening brace, possibly
		// after a space.
		rest := css[from:]
		open := strings.Index(rest, "{")
		if open < 0 {
			return found
		}
		if strings.TrimSpace(rest[:open]) != "" {
			continue
		}
		// The character before must not continue a selector: ".header-search"
		// must not match inside ".x .header-search" or "-results".
		if at > 0 {
			prev := css[at-1]
			if prev != ' ' && prev != '\t' && prev != '\n' && prev != '{' && prev != '}' && prev != ',' {
				continue
			}
			if prev == ' ' || prev == '\t' {
				// Indented: walk back to the line start and require only
				// whitespace before the selector.
				lineStart := at
				for lineStart > 0 && css[lineStart-1] != '\n' {
					lineStart--
				}
				if strings.TrimSpace(css[lineStart:at]) != "" {
					continue
				}
			}
		}
		found = append(found, at)
	}
}

// ruleBlock returns the text of the rule starting at at.
func ruleBlock(css string, at int) string {
	open := strings.Index(css[at:], "{")
	depth := 0
	for i := at + open; i < len(css); i++ {
		switch css[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return css[at : i+1]
			}
		}
	}
	return css[at:]
}

// TestThePhoneSearchRulesComeAfterTheDesktopOnes is the ordering rule that was
// broken: a media query has no extra specificity, so an override must follow
// the rule it overrides.
func TestThePhoneSearchRulesComeAfterTheDesktopOnes(t *testing.T) {
	css := readAsset(t, "css/browser.css")

	rules := ruleIndex(css, ".header-search-results")
	if len(rules) < 2 {
		t.Fatalf("expected a desktop and a phone .header-search-results rule, found %d", len(rules))
	}
	winning := ruleBlock(css, rules[len(rules)-1])
	for _, want := range []string{"transform: none", "left: 0", "right: 0", "max-width: none"} {
		if !strings.Contains(winning, want) {
			t.Errorf("the winning .header-search-results rule is missing %q; the desktop rule is still overriding the phone one", want)
		}
	}
	// It must be the phone rule, inside a max-width media query, or a desktop
	// would get a full-bleed panel.
	media := strings.LastIndex(css[:rules[len(rules)-1]], "@media (max-width: 720px)")
	if media < 0 {
		t.Fatal("the winning rule is not inside the phone media query")
	}
}

// TestThePhoneSearchPanelIsAnchoredToTheHeader pins the containing block: the
// panel hangs off the header, which is always the width of the card, not off
// the field, which is only the space left over.
func TestThePhoneSearchPanelIsAnchoredToTheHeader(t *testing.T) {
	css := readAsset(t, "css/browser.css")

	for _, selector := range []string{".browser-view .app-header", ".header-search"} {
		rules := ruleIndex(css, selector)
		if len(rules) == 0 {
			t.Fatalf("no rule for %s", selector)
		}
		winning := ruleBlock(css, rules[len(rules)-1])
		want := "position: relative"
		if selector == ".header-search" {
			want = "position: static"
		}
		if !strings.Contains(winning, want) {
			t.Errorf("the last rule for %s should say %q; the phone panel is measured from the wrong box otherwise", selector, want)
		}
		media := strings.LastIndex(css[:rules[len(rules)-1]], "@media (max-width: 720px)")
		if media < 0 {
			t.Errorf("the rule setting %s for %s is not inside the phone media query", want, selector)
		}
	}
}

// TestTheHiddenLogoDoesNotEatTheHeaderGap: a display:none flex item still has
// the header's gap on either side of it, which cost the search field 32px on a
// phone.
func TestTheHiddenLogoDoesNotEatTheHeaderGap(t *testing.T) {
	css := readAsset(t, "css/browser.css")
	rules := ruleIndex(css, ".browser-view .app-header")
	if len(rules) == 0 {
		t.Fatal("no .browser-view .app-header rule")
	}
	winning := ruleBlock(css, rules[len(rules)-1])
	if !strings.Contains(winning, "gap: 0") {
		t.Error("the phone header keeps its gap although the logo is display:none, leaving an empty slot either side of it")
	}
}
