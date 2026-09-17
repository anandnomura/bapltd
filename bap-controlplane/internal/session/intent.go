package session

import (
	"sort"
	"strings"
)

const IntentClassifierVersion = "bap-intent-rules-v1"

type intentRule struct {
	Category string
	Phrases  []intentPhrase
}

type intentPhrase struct {
	Text   string
	Label  string
	Weight int
}

type intentTagRule struct {
	Tag     string
	Phrases []string
}

var intentRules = []intentRule{
	{Category: "BUG_FIX", Phrases: []intentPhrase{{" fix bug ", "fix bug", 8}, {" bug fix ", "bug fix", 8}, {" fix the bug ", "fix the bug", 8}, {" bugfix ", "bugfix", 8}, {" regression ", "regression", 5}, {" broken ", "broken", 4}, {" defect ", "defect", 4}, {" bug ", "bug", 4}, {" failing ", "failing", 3}, {" error ", "error", 2}, {" fix ", "fix", 2}}},
	{Category: "DATABASE_CHANGE", Phrases: []intentPhrase{{" update db ", "update db", 7}, {" update database ", "update database", 7}, {" database migration ", "database migration", 7}, {" migrate database ", "migrate database", 7}, {" schema change ", "schema change", 6}, {" alter table ", "alter table", 6}, {" database ", "database", 3}, {" db ", "db", 3}, {" sql ", "sql", 2}}},
	{Category: "FEATURE_ENHANCEMENT", Phrases: []intentPhrase{{" new feature ", "new feature", 7}, {" add feature ", "add feature", 6}, {" enhance ui ", "enhance ui", 6}, {" ui enhancement ", "ui enhancement", 6}, {" enhancement ", "enhancement", 4}, {" enhance ", "enhance", 4}, {" improve ", "improve", 3}, {" implement ", "implement", 3}, {" add ", "add", 2}, {" build ", "build", 2}, {" create ", "create", 2}}},
	{Category: "INVESTIGATION", Phrases: []intentPhrase{{" root cause ", "root cause", 7}, {" investigate ", "investigate", 6}, {" analyze ", "analyze", 5}, {" diagnose ", "diagnose", 5}, {" find out ", "find out", 4}, {" understand ", "understand", 3}, {" inspect ", "inspect", 2}, {" review ", "review", 2}, {" reconcile ", "reconcile", 4}, {" audit ", "audit", 3}}},
	{Category: "REFACTOR", Phrases: []intentPhrase{{" refactor ", "refactor", 7}, {" clean up code ", "clean up code", 5}, {" restructure ", "restructure", 5}, {" simplify code ", "simplify code", 4}, {" optimize ", "optimize", 3}}},
	{Category: "TEST_VERIFICATION", Phrases: []intentPhrase{{" write tests ", "write tests", 7}, {" add tests ", "add tests", 7}, {" test coverage ", "test coverage", 6}, {" verify ", "verify", 4}, {" validate ", "validate", 4}, {" test ", "test", 3}}},
	{Category: "DOCUMENTATION", Phrases: []intentPhrase{{" update readme ", "update readme", 7}, {" write docs ", "write docs", 7}, {" documentation ", "documentation", 6}, {" document ", "document", 5}, {" readme ", "readme", 4}}},
	{Category: "MIGRATION", Phrases: []intentPhrase{{" dependency update ", "dependency update", 7}, {" upgrade dependency ", "upgrade dependency", 7}, {" upgrade framework ", "upgrade framework", 7}, {" migration ", "migration", 5}, {" migrate ", "migrate", 5}, {" modernize ", "modernize", 4}}},
	{Category: "DEPLOYMENT_RELEASE", Phrases: []intentPhrase{{" deploy to production ", "deploy to production", 8}, {" production deployment ", "production deployment", 8}, {" deployment ", "deployment", 5}, {" release ", "release", 5}, {" deploy ", "deploy", 5}, {" rollout ", "rollout", 4}, {" build pipeline ", "build pipeline", 3}}},
	{Category: "WORK_MANAGEMENT", Phrases: []intentPhrase{{" jira ", "jira", 6}, {" create ticket ", "create ticket", 6}, {" update ticket ", "update ticket", 6}, {" pull request ", "pull request", 5}, {" open pr ", "open pr", 5}, {" create pr ", "create pr", 5}, {" work item ", "work item", 4}}},
	{Category: "SECURITY_REMEDIATION", Phrases: []intentPhrase{{" security fix ", "security fix", 8}, {" remediate vulnerability ", "remediate vulnerability", 8}, {" vulnerability ", "vulnerability", 6}, {" cve ", "cve", 6}, {" security ", "security", 3}, {" permission ", "permission", 2}, {" probe ", "probe", 3}, {" credential ", "credential", 3}}},
}

var intentTagRules = []intentTagRule{
	{Tag: "API", Phrases: []string{" api ", " endpoint ", " rest ", " graphql "}},
	{Tag: "CUSTOMER_FACING", Phrases: []string{" customer ", " client facing ", " user facing "}},
	{Tag: "DATABASE", Phrases: []string{" db ", " database ", " schema ", " sql ", " table "}},
	{Tag: "INFRASTRUCTURE", Phrases: []string{" terraform ", " kubernetes ", " k8s ", " cloud ", " infrastructure "}},
	{Tag: "PRODUCTION", Phrases: []string{" production ", " prod ", " release ", " deploy "}},
	{Tag: "SECURITY", Phrases: []string{" security ", " vulnerability ", " cve ", " credential ", " permission ", " secret "}},
	{Tag: "UI", Phrases: []string{" ui ", " frontend ", " dashboard ", " screen ", " css ", " ux "}},
}

func normalizedIntentText(prompt string) string {
	lower := strings.ToLower(prompt)
	var b strings.Builder
	b.Grow(len(lower) + 2)
	b.WriteByte(' ')
	lastSpace := true
	for _, r := range lower {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastSpace = false
		} else if !lastSpace {
			b.WriteByte(' ')
			lastSpace = true
		}
	}
	if !lastSpace {
		b.WriteByte(' ')
	}
	return b.String()
}

// ClassifyIntent deterministically classifies a prompt string into a canonical IntentContext.
func ClassifyIntent(prompt string) IntentContext {
	result := IntentContext{
		Primary:           "UNKNOWN",
		Confidence:        0,
		ClassifierVersion: IntentClassifierVersion,
		Source:            "bap-server-classifier",
	}
	normalized := normalizedIntentText(prompt)
	if strings.TrimSpace(normalized) == "" {
		result.Evidence = []string{"empty-or-unavailable-prompt"}
		return result
	}

	type scoredIntent struct {
		category string
		score    int
		order    int
	}
	scored := make([]scoredIntent, 0, len(intentRules))
	for order, rule := range intentRules {
		score := 0
		for _, phrase := range rule.Phrases {
			if strings.Contains(normalized, phrase.Text) {
				score += phrase.Weight
				result.Evidence = append(result.Evidence, rule.Category+":"+phrase.Label)
			}
		}
		if rule.Category == "BUG_FIX" && strings.Contains(normalized, " fix ") && strings.Contains(normalized, " bug ") {
			score += 8
			result.Evidence = append(result.Evidence, "BUG_FIX:fix+bug")
		}
		if score > 0 {
			scored = append(scored, scoredIntent{category: rule.Category, score: score, order: order})
		}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].order < scored[j].order
		}
		return scored[i].score > scored[j].score
	})
	if len(scored) > 0 {
		result.Primary = scored[0].category
		confidence := 0.62 + float64(scored[0].score)*0.04
		if confidence > 0.98 {
			confidence = 0.98
		}
		if len(scored) > 1 && scored[0].score-scored[1].score <= 1 && confidence > 0.72 {
			confidence = 0.72
		}
		result.Confidence = confidence
		for _, item := range scored[1:] {
			if item.score >= 2 && len(result.Secondary) < 3 {
				result.Secondary = append(result.Secondary, item.category)
			}
		}
	} else {
		result.Evidence = append(result.Evidence, "no-rule-match")
	}

	for _, rule := range intentTagRules {
		for _, phrase := range rule.Phrases {
			if strings.Contains(normalized, phrase) {
				result.Tags = append(result.Tags, rule.Tag)
				break
			}
		}
	}
	return result
}
