package jirarest

import (
	"encoding/json"
	"os"
	"strconv"

	jira "github.com/andygrunwald/go-jira"
	"github.com/grokify/gocharts/v2/data/histogram"
	"github.com/grokify/gojira"
	"github.com/grokify/mogo/encoding/jsonutil"
)

type Issues []jira.Issue

func (ii Issues) CountsByType() map[string]int {
	counts := map[string]int{}
	for _, iss := range ii {
		name := iss.Fields.Type.Name
		counts[name]++
		counts["_total"]++
	}
	return counts
}

// CountsByProjectTypeStatus returns a `*histogram.Histogram` with issue counts
// by project, type, and status. This can be used to export CSV and XLSX sheets
// for analysis.
func (ii Issues) CountsByProjectTypeStatus() *histogram.HistogramSets {
	hsets := histogram.NewHistogramSets("")
	for _, iss := range ii {
		hsets.Add(
			iss.Fields.Project.Key,
			iss.Fields.Type.Name,
			iss.Fields.Status.Name,
			1,
			true)
	}
	return hsets
}

func (ii Issues) AddRank() Issues {
	nii := Issues{}
	for i, iss := range ii {
		if iss.Fields == nil {
			iss.Fields = &jira.IssueFields{}
		}
		iss.Fields.Unknowns[MetaParamRank] = strconv.Itoa(i)
		nii = append(nii, iss)
	}
	return nii
}

func (ii Issues) IssuesSet(cfg *gojira.Config) (*IssuesSet, error) {
	is := NewIssuesSet(cfg)
	err := is.Add(ii...)
	return is, err
}

func (ii Issues) WriteFileJSON(filename, prefix, indent string) error {
	b, err := jsonutil.MarshalSimple(ii, prefix, indent)
	if err != nil {
		return err
	}
	return os.WriteFile(filename, b, 0600)
}

func IssuesReadFileJSON(filename string) (Issues, error) {
	ii := Issues{}
	b, err := os.ReadFile(filename)
	if err != nil {
		return ii, err
	}
	return ii, json.Unmarshal(b, &ii)
}
