// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package spec

import (
	"strings"
	"unicode"
)

// UpperFirst uppercases the first rune of s.
func UpperFirst(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// ToGroupGoName derives the Go-facing group name from a short group name,
// mirroring client-gen's groupGoName: take the first path segment, capitalize
// it. For example "apps" -> "Apps", "apps.example.com" -> "Apps".
func ToGroupGoName(group string) string {
	if group == "" {
		return "Core"
	}
	seg := group
	if i := strings.Index(seg, "."); i >= 0 {
		seg = seg[:i]
	}
	return UpperFirst(strings.ToLower(seg))
}

// Pluralize derives a plural REST name from a singular name using a small set
// of English rules plus an exceptions table. It intentionally mirrors the
// common Kubernetes plural conventions; callers may override via the Resource
// option when the result is wrong.
func Pluralize(singular string) string {
	if singular == "" {
		return ""
	}
	lower := strings.ToLower(singular)
	if p, ok := irregularPlurals[lower]; ok {
		return p
	}
	switch {
	case strings.HasSuffix(lower, "s"),
		strings.HasSuffix(lower, "x"),
		strings.HasSuffix(lower, "z"),
		strings.HasSuffix(lower, "ch"),
		strings.HasSuffix(lower, "sh"):
		return singular + "es"
	case strings.HasSuffix(lower, "y") && len(singular) > 1 && isConsonant(singular[len(singular)-2]):
		return singular[:len(singular)-1] + "ies"
	case strings.HasSuffix(lower, "f"):
		return singular[:len(singular)-1] + "ves"
	case strings.HasSuffix(lower, "fe"):
		return singular[:len(singular)-2] + "ves"
	default:
		return singular + "s"
	}
}

func isConsonant(b byte) bool {
	switch b {
	case 'a', 'e', 'i', 'o', 'u', 'y':
		return false
	default:
		return true
	}
}

// irregularPlurals holds plural overrides for kinds that do not follow the
// simple rules above. It covers two cases:
//  1. Kubernetes kinds that are already plural in form (e.g. Endpoints), where
//     the singular name equals the plural.
//  2. Common irregular English plurals that a user-defined resource kind may
//     collide with.
var irregularPlurals = map[string]string{
	// Kubernetes kinds that are themselves plural.
	"endpoints": "endpoints",

	// Common irregular English plurals (also relevant to compound kinds).
	"policy":                "policies",
	"category":              "categories",
	"entry":                 "entries",
	"endpoint":              "endpoints",
	"ingressclass":          "ingressclasses",
	"networkpolicy":         "networkpolicies",
	"priorityclass":         "priorityclasses",
	"storageclass":          "storageclasses",
	"volumeattachmentclass": "volumeattachmentclasses",

	"child":      "children",
	"person":     "people",
	"man":        "men",
	"woman":      "women",
	"foot":       "feet",
	"tooth":      "teeth",
	"mouse":      "mice",
	"goose":      "geese",
	"quiz":       "quizzes",
	"index":      "indices",
	"matrix":     "matrices",
	"vertex":     "vertices",
	"axis":       "axes",
	"analysis":   "analyses",
	"basis":      "bases",
	"crisis":     "crises",
	"thesis":     "theses",
	"datum":      "data",
	"medium":     "media",
	"criterion":  "criteria",
	"phenomenon": "phenomena",
	"series":     "series",
	"species":    "species",
	"sheep":      "sheep",
	"deer":       "deer",
	"fish":       "fish",
}
