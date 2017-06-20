package utils

import (
	"encoding/json"
	"github.com/jfrogdev/jfrog-cli-go/utils/ioutils"
	"github.com/jfrogdev/jfrog-cli-go/utils/cliutils"
	"strings"
	"strconv"
)

const (
	WILDCARD SpecType = "wildcard"
	SIMPLE SpecType = "simple"
	AQL SpecType = "aql"
)

type Aql struct {
	ItemsFind string `json:"items.find"`
}

type Files struct {
	Pattern   string
	Target    string
	Props     string
	Recursive string
	Flat      string
	Regexp    string
	Aql       Aql
}

type SpecFiles struct {
	Files []Files
}

func (spec *SpecFiles) Get(index int) *Files {
	if index < len(spec.Files) {
		return &spec.Files[index]
	}
	return new(Files)
}

func (aql *Aql) UnmarshalJSON(value []byte) error {
	str := string(value)
	first := strings.Index(str[strings.Index(str, "{") + 1 :], "{")
	last := strings.LastIndex(str, "}")

	aql.ItemsFind = cliutils.StripChars(str[first:last], "\n\t ")
	return nil
}

func CreateSpecFromFile(specFilePath string) (spec *SpecFiles, err error) {
	spec = new(SpecFiles)
	content, err := ioutils.ReadFile(specFilePath)
	if cliutils.CheckError(err) != nil {
		return
	}

	err = json.Unmarshal(content, spec)
	if cliutils.CheckError(err) != nil {
		return
	}
	return
}

func CreateSpec(pattern, target, props string, recursive, flat, regexp bool) (spec *SpecFiles) {
	spec = &SpecFiles{
		Files: []Files{
			{
				Pattern:   pattern,
				Target:    target,
				Props:     props,
				Recursive: strconv.FormatBool(recursive),
				Flat:      strconv.FormatBool(flat),
				Regexp:    strconv.FormatBool(regexp),
			},
		},
	}
	return spec
}

func (files Files) GetSpecType() (specType SpecType) {
	switch {
	case files.Pattern != "" && IsWildcardPattern(files.Pattern):
		specType = WILDCARD
	case files.Pattern != "":
		specType = SIMPLE
	case files.Aql.ItemsFind != "" :
		specType = AQL
	}
	return specType
}

type SpecType string