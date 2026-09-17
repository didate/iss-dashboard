package typologie

import (
	"sort"
	"strings"

	"iss-dashboard-backend/internal/models"
)

// Result is the resolved type of one structure.
type Result struct {
	Code   string   // one of the type codes
	Source string   // SourceGroup | SourceGroupMultiple | SourceName | SourceNone
	Groups []string // typology group names the org unit belongs to (for messages)
}

// Index resolves types and ownership from org unit group memberships.
type Index struct {
	typologyByOU  map[string][]string // ou → typology group names
	hospitalByOU  map[string]string   // ou → hospital subtype code
	ownershipByOU map[string]string   // ou → "publique" | "privée"
}

// NewIndex builds an Index from raw memberships and the configured set names.
// Set names are compared case-insensitively after trimming.
func NewIndex(memberships []models.OrgUnitGroup, typologySet, hospitalSet, ownershipSet string) *Index {
	idx := &Index{
		typologyByOU:  make(map[string][]string),
		hospitalByOU:  make(map[string]string),
		ownershipByOU: make(map[string]string),
	}
	eq := func(a, b string) bool { return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b)) }
	seen := make(map[string]bool) // ou|group, memberships may repeat across sets
	for _, m := range memberships {
		switch {
		case eq(m.SetName, typologySet):
			key := m.OrgUnit + "|" + m.GroupUID
			if !seen[key] {
				seen[key] = true
				idx.typologyByOU[m.OrgUnit] = append(idx.typologyByOU[m.OrgUnit], m.GroupName)
			}
		case eq(m.SetName, hospitalSet):
			if sub := hospitalSubtype(m.GroupName); sub != "" {
				idx.hospitalByOU[m.OrgUnit] = sub
			}
		case eq(m.SetName, ownershipSet):
			if own := ownershipFromGroup(m.GroupName); own != "" {
				idx.ownershipByOU[m.OrgUnit] = own
			}
		}
	}
	for ou := range idx.typologyByOU {
		sort.Strings(idx.typologyByOU[ou])
	}
	return idx
}

// Ownership returns "publique", "privée" or "" (org unit in neither group).
func (idx *Index) Ownership(ouUID string) string {
	if idx == nil {
		return ""
	}
	return idx.ownershipByOU[ouUID]
}

// Resolve determines the type of an org unit: typology group first (refined by the
// hospital set), then name prefix, then INDETERMINE. A private structure that no
// group nor prefix classifies becomes AUTRE_PRIVE rather than INDETERMINE.
func (idx *Index) Resolve(ou models.OrgUnit) Result {
	var groups []string
	if idx != nil {
		groups = idx.typologyByOU[ou.UID]
	}

	var codes []string
	for _, g := range groups {
		if code, ok := groupNameToCode[normalizeGroupName(g)]; ok {
			if code == HP && idx != nil {
				if sub, ok := idx.hospitalByOU[ou.UID]; ok {
					code = sub
				}
			}
			codes = append(codes, code)
		}
	}

	switch len(codes) {
	case 1:
		return Result{Code: codes[0], Source: SourceGroup, Groups: groups}
	case 0:
		// fall through to name-based resolution
	default:
		return Result{Code: pickByPriority(codes), Source: SourceGroupMultiple, Groups: groups}
	}

	if code := codeFromName(ou.Name); code != "" {
		return Result{Code: code, Source: SourceName, Groups: groups}
	}
	if idx != nil && idx.ownershipByOU[ou.UID] == "privée" {
		return Result{Code: AutrePrive, Source: SourceName, Groups: groups}
	}
	return Result{Code: Indetermine, Source: SourceNone, Groups: groups}
}

func pickByPriority(codes []string) string {
	for _, p := range groupPriority {
		for _, c := range codes {
			if c == p {
				return c
			}
		}
	}
	return codes[0]
}
