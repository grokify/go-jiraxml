package jirarest

import (
	"github.com/grokify/mogo/type/maputil"
	"github.com/grokify/mogo/type/stringsutil"
)

func (is *IssuesSet) Types(inclBase, inclParents bool) []string {
	var types []string
	if inclBase {
		t := is.types()
		types = append(types, t...)
	}
	if inclParents && is.Parents != nil {
		t := is.Parents.Types(inclParents, inclParents)
		// t := is.Parents.types()
		types = append(types, t...)
	}
	return stringsutil.SliceCondenseSpace(types, true, true)
}

func (is *IssuesSet) types() []string {
	types := map[string]int{}
	for _, iss := range is.IssuesMap {
		iss := iss
		im := NewIssueMore(&iss)
		types[im.Type()]++
	}
	return maputil.Keys(types)
}

func (is *IssuesSet) KeysForTypes(types []string, inclBase, inclParents bool) []string {
	if len(types) == 0 ||
		(!inclBase && !inclParents) {
		return []string{}
	}
	var keys []string
	if inclBase {
		k := is.keysForTypes(types)
		keys = append(keys, k...)
	}
	if inclParents && is.Parents != nil {
		k := is.Parents.KeysForTypes(types, inclParents, inclParents)
		// k := is.Parents.keysForTypes(types)
		keys = append(keys, k...)
		if inclBase {
			keys = stringsutil.SliceCondenseSpace(keys, true, true)
		}
	}
	return keys
}

func (is *IssuesSet) keysForTypes(types []string) []string {
	if len(types) == 0 {
		return []string{}
	}
	typeMap := map[string]int{}
	for _, t := range types {
		typeMap[t]++
	}
	var keys []string
	for _, iss := range is.IssuesMap {
		iss := iss
		im := NewIssueMore(&iss)
		t := im.Type()
		if _, ok := typeMap[t]; ok {
			keys = append(keys, im.Key())
		}
	}
	return stringsutil.SliceCondenseSpace(keys, true, true)
}
