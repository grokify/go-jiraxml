package jirarest

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	jira "github.com/andygrunwald/go-jira"
	"github.com/grokify/gojira"
	"github.com/grokify/mogo/encoding/jsonutil"
	"github.com/grokify/mogo/errors/errorsutil"
	"github.com/grokify/mogo/type/maputil"
	"github.com/grokify/mogo/type/slicesutil"
)

type IssuesSet struct {
	Config    *gojira.Config
	IssuesMap map[string]jira.Issue
	Parents   *IssuesSet
}

func NewIssuesSet(cfg *gojira.Config) *IssuesSet {
	if cfg == nil {
		cfg = gojira.NewConfigDefault()
	}
	return &IssuesSet{
		Config:    cfg,
		IssuesMap: map[string]jira.Issue{},
		Parents: &IssuesSet{
			Config:    cfg,
			IssuesMap: map[string]jira.Issue{},
		},
	}
}

// StatusesOrder returns the status order from `StageConfig{}`.
func (set *IssuesSet) StatusesOrder() []string {
	if set.Config != nil && set.Config.StatusConfig != nil {
		return set.Config.StatusConfig.StageConfig.Order()
	} else {
		return []string{}
	}
}

// AddIssuesFile reads a `Issues{}` JSON file and adds it to the `IssuesSet{}`.
func (set *IssuesSet) AddIssuesFile(filename string) error {
	if ii, err := IssuesReadFileJSON(filename); err != nil {
		return err
	} else {
		return set.Add(ii...)
	}
}

func (set *IssuesSet) Add(issues ...jira.Issue) error {
	if set.IssuesMap == nil {
		set.IssuesMap = map[string]jira.Issue{}
	}
	for _, iss := range issues {
		if key := strings.TrimSpace(iss.Key); key == "" {
			return errors.New("no key")
		} else {
			set.IssuesMap[key] = iss
		}
	}
	return nil
}

// IssueFirst returns the first issue by alphabetical sorting of keys.
// It is primarily used for testing purposes to get an issue.
func (set *IssuesSet) IssueFirst() (jira.Issue, error) {
	keys := set.Keys()
	if len(keys) == 0 {
		return jira.Issue{}, errors.New("no issues present")
	} else if iss, ok := set.IssuesMap[keys[0]]; ok {
		return iss, nil
	} else {
		panic(fmt.Sprintf("issue key from map not found (%s)", keys[0]))
	}
}

// KeyExists returns a boolean representing the existence of an issue key.
func (set *IssuesSet) KeyExists(key string, inclParents bool) bool {
	if _, ok := set.IssuesMap[key]; ok {
		return true
	} else if !inclParents || set.Parents == nil {
		return false
	} else {
		return set.Parents.KeyExists(key, inclParents)
	}
}

// Keys returns a slice of sorted issue keys.
func (set *IssuesSet) Keys() []string             { return maputil.Keys(set.IssuesMap) }
func (set *IssuesSet) Len() int                   { return len(set.IssuesMap) }
func (set *IssuesSet) LenParents() int            { return len(set.KeysParents()) }
func (set *IssuesSet) LenParentsPopulated() int   { return len(set.KeysParentsPopulated()) }
func (set *IssuesSet) LenParentsUnpopulated() int { return len(set.KeysParentsUnpopulated()) }

func (set *IssuesSet) LenLineageTopKeysPopulated() int {
	if linPopIDs, err := set.LineageTopKeysPopulated(); err != nil {
		panic(err)
	} else {
		return len(linPopIDs)
	}
}

func (set *IssuesSet) LenLineageTopKeysUnpopulated() int {
	if linUnpopIDs, err := set.LineageTopKeysUnpopulated(); err != nil {
		panic(err)
	} else {
		return len(linUnpopIDs)
	}
}

// LenMap provides various metrics. It is useful for determining if all parents and lineages have been loaded.
func (set *IssuesSet) LenMap() map[string]int {
	lenParentsSet := 0
	if set.Parents != nil {
		lenParentsSet = len(set.Parents.IssuesMap)
	}
	return map[string]int{
		"len":                       set.Len(),
		"lineageTopKeysPopulated":   set.LenLineageTopKeysPopulated(),
		"lineageTopKeysUnpopulated": set.LenLineageTopKeysUnpopulated(),
		"parents":                   set.LenParents(),
		"parentsPopulated":          set.LenParentsPopulated(),
		"parentsUnpopulated":        set.LenParentsUnpopulated(),
		"parentsSetAll":             lenParentsSet,
	}
}

func (set *IssuesSet) EpicKeys(customFieldID string) []string {
	var keys []string
	for _, iss := range set.IssuesMap {
		if iss.Fields == nil {
			continue
		}
		if iss.Fields.Epic != nil {
			keys = append(keys, iss.Fields.Epic.Key)
		}
		epickey := IssueFieldsCustomFieldString(iss.Fields, customFieldID)
		if epickey != "" {
			keys = append(keys, epickey)
		}
	}
	keys = slicesutil.Dedupe(keys)
	sort.Strings(keys)
	return keys
}

func (set *IssuesSet) InflateEpicKeys(customFieldEpicLinkID string) {
	for k, iss := range set.IssuesMap {
		if iss.Fields == nil {
			continue
		}
		if iss.Fields.Epic != nil && strings.TrimSpace(iss.Fields.Epic.Key) != "" {
			continue
		}
		epicKey := IssueFieldsCustomFieldString(iss.Fields, customFieldEpicLinkID)
		if epicKey != "" {
			if iss.Fields.Epic == nil {
				iss.Fields.Epic = &jira.Epic{}
			}
			iss.Fields.Epic.Key = epicKey
		}
		set.IssuesMap[k] = iss
	}
}

// InflateEpics uses the Jira REST API to inflate the Issue struct with an Epic struct.
func (set *IssuesSet) InflateEpics(jclient *jira.Client, customFieldIDEpicLink string) error {
	epicKeys := set.EpicKeys(customFieldIDEpicLink)
	var newEpicKeys []string
	for _, key := range epicKeys {
		if _, ok := set.IssuesMap[key]; !ok {
			newEpicKeys = append(newEpicKeys, key)
		}
	}
	epicsSet := NewEpicsSet()
	err := epicsSet.GetKeys(jclient, newEpicKeys)
	if err != nil {
		return err
	}

	for k, iss := range set.IssuesMap {
		issEpicKey := strings.TrimSpace(IssueFieldsCustomFieldString(iss.Fields, customFieldIDEpicLink))
		if issEpicKey == "" {
			continue
		}
		epic, ok := epicsSet.EpicsMap[issEpicKey]
		if !ok {
			panic("not found")
		}
		iss.Fields.Epic = &epic
		set.IssuesMap[k] = iss
	}
	return nil
}

// Issue returns a `jira.Issue` given an issue key.
func (set *IssuesSet) Issue(key string) (jira.Issue, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return jira.Issue{}, errors.New("key not provided")
	}
	if iss, ok := set.IssuesMap[key]; ok {
		return iss, nil
	} else if set.Parents != nil {
		if iss, ok := set.Parents.IssuesMap[key]; ok {
			return iss, nil
		}
	}
	return jira.Issue{}, errors.New("key not found")
}

// Issues returns the issues in the set as an `Issues{}` slice.
func (set *IssuesSet) Issues(keys ...string) Issues {
	var ii Issues
	if len(keys) == 0 {
		for _, iss := range set.IssuesMap {
			ii = append(ii, iss)
		}
	} else {
		for _, key := range keys {
			if iss, ok := set.IssuesMap[key]; ok {
				ii = append(ii, iss)
			}
		}
	}
	return ii
}

func (set *IssuesSet) IssueMetas(customFieldLabels []string) IssueMetas {
	var imetas IssueMetas
	for _, iss := range set.IssuesMap {
		iss := iss
		issMore := NewIssueMore(&iss)
		issMeta := issMore.Meta(set.Config.ServerURL, customFieldLabels)
		imetas = append(imetas, issMeta)
	}
	return imetas
}

func (set *IssuesSet) IssueMores(keys ...string) IssueMores {
	var ims IssueMores
	if len(keys) == 0 {
		for _, iss := range set.IssuesMap {
			im := NewIssueMore(&iss)
			ims = append(ims, im)
		}
	} else {
		for _, k := range keys {
			k = strings.TrimSpace(k)
			if k == "" {
				continue
			} else if iss, ok := set.IssuesMap[k]; ok {
				im := NewIssueMore(&iss)
				ims = append(ims, im)
			}
		}
	}
	return ims
}

func (set *IssuesSet) IssuesSetHighestType(issueType string) (*IssuesSet, error) {
	new := NewIssuesSet(set.Config)
	for _, iss := range set.IssuesMap {
		iss := iss
		issMore := NewIssueMore(&iss)
		issMeta := issMore.Meta(set.Config.ServerURL, []string{})
		issKey := strings.TrimSpace(issMeta.Key)
		if issKey != "" {
			lineage, err := set.Lineage(issKey, []string{})
			if err != nil {
				return nil, errorsutil.Wrapf(err, "error on `is.Lineage(%s)`", issKey)
			}
			if issMetaType := lineage.HighestType(issueType); issMetaType != nil && strings.TrimSpace(issMetaType.Key) != "" {
				if issType, err := set.Issue(issMetaType.Key); err != nil {
					return nil, errorsutil.Wrapf(err, "error on `is.Get(%s)`", issMetaType.Key)
				} else {
					if err := new.Add(issType); err != nil {
						return nil, err
					}
				}
			}
		}
	}
	new.Parents = set.Parents
	return new, nil
}

// WriteFileJSON writes the `IssuesSet{}` as a JSON file.
func (set *IssuesSet) WriteFileJSON(name, prefix, indent string) error {
	j, err := jsonutil.MarshalSimple(set, prefix, indent)
	if err != nil {
		return err
	}
	return os.WriteFile(name, j, 0600)
}
