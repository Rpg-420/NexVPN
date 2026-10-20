package parser

import (
	"strings"

	"nexvpn/internal/config"
)

// userRuleGroup collects every enabled user rule that shares a (kind, action)
// pair.
//
// Grouping is what keeps the generated config small: a user with thirty
// "direct" domains produces one route rule with thirty entries rather than
// thirty rules, which matters because every route rule is evaluated in order
// for every connection the core sees.
type userRuleGroup struct {
	Kind   string
	Action string
	Values []string
}

// groupUserRoutingRules returns the enabled rules grouped by kind and action,
// preserving first-seen order so the generated config is stable between runs
// (an unstable config would make every reconnect look like a config change).
func groupUserRoutingRules(rules []config.RoutingRule) []userRuleGroup {
	index := make(map[string]int, len(rules))
	groups := make([]userRuleGroup, 0, len(rules))

	for _, rule := range rules {
		if !rule.Enabled || strings.TrimSpace(rule.Value) == "" {
			continue
		}

		key := rule.Kind + "\x00" + rule.Action
		position, ok := index[key]
		if !ok {
			position = len(groups)
			index[key] = position
			groups = append(groups, userRuleGroup{Kind: rule.Kind, Action: rule.Action})
		}

		value := rule.Value
		if rule.Kind == config.RoutingKindIP {
			// A bare IP has to be written as a single-host prefix: both cores
			// treat an unprefixed address in an ip_cidr/ip list inconsistently,
			// and the v6 form needs a different length.
			if strings.Contains(value, ":") {
				value += "/128"
			} else {
				value += "/32"
			}
		}
		groups[position].Values = append(groups[position].Values, value)
	}

	return groups
}

// outboundFor maps a rule action onto the outbound tag used by the generated
// configuration.
//
// One function serves both cores: sing-box names its proxy outbound after the
// profile's tag and Xray names it "proxy", so the difference is entirely in the
// proxyTag the caller passes rather than in the mapping itself.
func outboundFor(action, proxyTag string) string {
	switch action {
	case config.RoutingActionDirect:
		return "direct"
	case config.RoutingActionBlock:
		return "block"
	default:
		return proxyTag
	}
}

// UserRoutingRulesSingBox converts the user's routing overrides into sing-box
// route rules.
//
// These are appended after the built-in mode rules and before the final
// catch-all, so a manual entry always wins over the built-in split — which is
// the only ordering that makes a manual override behave like one.
func UserRoutingRulesSingBox(rules []config.RoutingRule, proxyTag string) []map[string]interface{} {
	groups := groupUserRoutingRules(rules)
	if len(groups) == 0 {
		return nil
	}

	out := make([]map[string]interface{}, 0, len(groups))
	for _, group := range groups {
		rule := map[string]interface{}{
			"outbound": outboundFor(group.Action, proxyTag),
		}
		if group.Kind == config.RoutingKindDomain {
			rule["domain"] = group.Values
		} else {
			rule["ip_cidr"] = group.Values
		}
		out = append(out, rule)
	}
	return out
}

// UserRoutingRulesXray converts the user's routing overrides into Xray routing
// rules.
//
// Domain values are emitted with the `domain:` prefix, which is Xray's
// suffix-match operator. A bare string would be a substring match, so
// `example.com` would also capture `notexample.com` — a rule that silently
// routes more than the user asked for.
func UserRoutingRulesXray(rules []config.RoutingRule, proxyTag string) []map[string]interface{} {
	groups := groupUserRoutingRules(rules)
	if len(groups) == 0 {
		return nil
	}

	out := make([]map[string]interface{}, 0, len(groups))
	for _, group := range groups {
		rule := map[string]interface{}{
			"type":        "field",
			"outboundTag": outboundFor(group.Action, proxyTag),
		}
		if group.Kind == config.RoutingKindDomain {
			values := make([]string, 0, len(group.Values))
			for _, value := range group.Values {
				values = append(values, "domain:"+value)
			}
			rule["domain"] = values
		} else {
			rule["ip"] = group.Values
		}
		out = append(out, rule)
	}
	return out
}
