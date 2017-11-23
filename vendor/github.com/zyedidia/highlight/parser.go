package highlight

import (
	"fmt"
	"regexp"

	"gopkg.in/yaml.v2"
)

// A Group represents a syntax group
type Group uint8

// Groups contains all of the groups that are defined
// You can access them in the map via their string name
var Groups map[string]Group
var numGroups Group

// String returns the group name attached to the specific group
func (g Group) String() string {
	for k, v := range Groups {
		if v == g {
			return k
		}
	}
	return ""
}

// A Def is a full syntax definition for a language
// It has a filetype, information about how to detect the filetype based
// on filename or header (the first line of the file)
// Then it has the rules which define how to highlight the file
type Def struct {
	FileType string
	ftdetect []*regexp.Regexp
	rules    *rules
}

// A Pattern is one simple syntax rule
// It has a group that the rule belongs to, as well as
// the regular expression to match the pattern
type pattern struct {
	group Group
	regex *regexp.Regexp
}

// rules defines which patterns and regions can be used to highlight
// a filetype
type rules struct {
	regions  []*region
	patterns []*pattern
	includes []string
}

// A region is a highlighted region (such as a multiline comment, or a string)
// It belongs to a group, and has start and end regular expressions
// A region also has rules of its own that only apply when matching inside the
// region and also rules from the above region do not match inside this region
// Note that a region may contain more regions
type region struct {
	group      Group
	limitGroup Group
	parent     *region
	start      *regexp.Regexp
	end        *regexp.Regexp
	skip       *regexp.Regexp
	rules      *rules
}

func init() {
	Groups = make(map[string]Group)
}

// ParseDef parses an input syntax file into a highlight Def
func ParseDef(input []byte) (s *Def, err error) {
	// This is just so if we have an error, we can exit cleanly and return the parse error to the user
	defer func() {
		if e := recover(); e != nil {
			err = e.(error)
		}
	}()

	var rules map[interface{}]interface{}
	if err = yaml.Unmarshal(input, &rules); err != nil {
		return nil, err
	}

	s = new(Def)

	for k, v := range rules {
		if k == "filetype" {
			filetype := v.(string)

			s.FileType = filetype
		} else if k == "detect" {
			ftdetect := v.(map[interface{}]interface{})
			if len(ftdetect) >= 1 {
				syntax, err := regexp.Compile(ftdetect["filename"].(string))
				if err != nil {
					return nil, err
				}

				s.ftdetect = append(s.ftdetect, syntax)
			}
			if len(ftdetect) >= 2 {
				header, err := regexp.Compile(ftdetect["header"].(string))
				if err != nil {
					return nil, err
				}

				s.ftdetect = append(s.ftdetect, header)
			}
		} else if k == "rules" {
			inputRules := v.([]interface{})

			rules, err := parseRules(inputRules, nil)
			if err != nil {
				return nil, err
			}

			s.rules = rules
		}
	}

	return s, err
}

// ResolveIncludes will sort out the rules for including other filetypes
// You should call this after parsing all the Defs
func ResolveIncludes(defs []*Def) {
	for _, d := range defs {
		resolveIncludesInDef(defs, d)
	}
}

func resolveIncludesInDef(defs []*Def, d *Def) {
	for _, lang := range d.rules.includes {
		for _, searchDef := range defs {
			if lang == searchDef.FileType {
				d.rules.patterns = append(d.rules.patterns, searchDef.rules.patterns...)
				d.rules.regions = append(d.rules.regions, searchDef.rules.regions...)
			}
		}
	}
	for _, r := range d.rules.regions {
		resolveIncludesInRegion(defs, r)
		r.parent = nil
	}
}

func resolveIncludesInRegion(defs []*Def, region *region) {
	for _, lang := range region.rules.includes {
		for _, searchDef := range defs {
			if lang == searchDef.FileType {
				region.rules.patterns = append(region.rules.patterns, searchDef.rules.patterns...)
				region.rules.regions = append(region.rules.regions, searchDef.rules.regions...)
			}
		}
	}
	for _, r := range region.rules.regions {
		resolveIncludesInRegion(defs, r)
		r.parent = region
	}
}

func parseRules(input []interface{}, curRegion *region) (*rules, error) {
	rules := new(rules)

	for _, v := range input {
		rule := v.(map[interface{}]interface{})
		for k, val := range rule {
			group := k

			switch object := val.(type) {
			case string:
				if k == "include" {
					rules.includes = append(rules.includes, object)
				} else {
					// Pattern
					r, err := regexp.Compile(object)
					if err != nil {
						return nil, err
					}

					groupStr := group.(string)
					if _, ok := Groups[groupStr]; !ok {
						numGroups++
						Groups[groupStr] = numGroups
					}
					groupNum := Groups[groupStr]
					rules.patterns = append(rules.patterns, &pattern{groupNum, r})
				}
			case map[interface{}]interface{}:
				// region
				region, err := parseRegion(group.(string), object, curRegion)
				if err != nil {
					return nil, err
				}
				rules.regions = append(rules.regions, region)
			default:
				return nil, fmt.Errorf("Bad type %T", object)
			}
		}
	}

	return rules, nil
}

func parseRegion(group string, regionInfo map[interface{}]interface{}, prevRegion *region) (*region, error) {
	var err error

	region := new(region)
	if _, ok := Groups[group]; !ok {
		numGroups++
		Groups[group] = numGroups
	}
	groupNum := Groups[group]
	region.group = groupNum
	region.parent = prevRegion

	region.start, err = regexp.Compile(regionInfo["start"].(string))

	if err != nil {
		return nil, err
	}

	region.end, err = regexp.Compile(regionInfo["end"].(string))

	if err != nil {
		return nil, err
	}

	// skip is optional
	if _, ok := regionInfo["skip"]; ok {
		region.skip, err = regexp.Compile(regionInfo["skip"].(string))

		if err != nil {
			return nil, err
		}
	}

	// limit-color is optional
	if _, ok := regionInfo["limit-group"]; ok {
		groupStr := regionInfo["limit-group"].(string)
		if _, ok := Groups[groupStr]; !ok {
			numGroups++
			Groups[groupStr] = numGroups
		}
		groupNum := Groups[groupStr]
		region.limitGroup = groupNum

		if err != nil {
			return nil, err
		}
	} else {
		region.limitGroup = region.group
	}

	region.rules, err = parseRules(regionInfo["rules"].([]interface{}), region)

	if err != nil {
		return nil, err
	}

	return region, nil
}
