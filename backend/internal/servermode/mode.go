package servermode

import "strings"

const (
	Public      = "public"
	Training    = "training"
	Competitive = "competitive"
)

type Resolution struct {
	Input      string
	Normalized string
	Reason     string
}

func Resolve(input string, legacyIsPublic *bool) Resolution {
	trimmed := strings.TrimSpace(input)
	normalized := strings.ToLower(trimmed)

	switch normalized {
	case Public:
		return Resolution{Input: trimmed, Normalized: Public, Reason: "explicit_public"}
	case Training:
		return Resolution{Input: trimmed, Normalized: Training, Reason: "explicit_training"}
	case Competitive:
		return Resolution{Input: trimmed, Normalized: Competitive, Reason: "explicit_competitive"}
	case "":
		if legacyIsPublic != nil {
			if *legacyIsPublic {
				return Resolution{Input: trimmed, Normalized: Public, Reason: "legacy_is_public_true"}
			}
			return Resolution{Input: trimmed, Normalized: Competitive, Reason: "legacy_is_public_false"}
		}
		return Resolution{Input: trimmed, Normalized: Competitive, Reason: "missing_default_competitive"}
	default:
		return Resolution{Input: trimmed, Normalized: Competitive, Reason: "invalid_default_competitive"}
	}
}

func IsCompetitive(mode string) bool {
	return strings.EqualFold(strings.TrimSpace(mode), Competitive)
}
