package main

import (
	"fmt"
	"regexp"

	"github.com/prymitive/karma/internal/alertmanager"
	"github.com/prymitive/karma/internal/config"
	"github.com/prymitive/karma/internal/models"
	"github.com/prymitive/karma/internal/slices"
)

const (
	aclActionRequireMatcher = "requireMatcher"
	aclActionBlock          = "block"
	aclActionAllow          = "allow"
)

var (
	allACLActions = []string{aclActionAllow, aclActionBlock, aclActionRequireMatcher}
)

type silenceFilter struct {
	Name       string
	NameRegex  *regexp.Regexp
	Value      string
	ValueRegex *regexp.Regexp
	IsRegex    bool
}

func (sf *silenceFilter) isMatch(silence *models.Silence) bool {
	for _, m := range silence.Matchers {
		var nameMatch bool
		if sf.Name != "" && sf.Name == m.Name {
			nameMatch = true
		} else if sf.NameRegex != nil && sf.NameRegex.MatchString(m.Name) {
			nameMatch = true
		}

		var valueMatch bool
		if sf.Value != "" && sf.Value == m.Value {
			valueMatch = true
		} else if sf.ValueRegex != nil && sf.ValueRegex.MatchString(m.Value) {
			valueMatch = true
		}

		if nameMatch && valueMatch && sf.IsRegex == m.IsRegex {
			return true
		}
	}

	return false
}

type silenceMatcher struct {
	Name    string
	Value   string
	IsRegex bool
}

type aclMatchers struct {
	Required []silenceMatcher
}

type silenceACLScope struct {
	Groups        []string
	Alertmanagers []string
	Filters       []silenceFilter
}

type silenceACL struct {
	Action   string
	Reason   string
	Scope    silenceACLScope
	Matchers aclMatchers
}

func (acl *silenceACL) isAllowed(amName string, silence *models.Silence, username string) (bool, error) {
	groups := userGroups(username)

	groupMatch := len(acl.Scope.Groups) == 0
	for _, aclGroup := range acl.Scope.Groups {
		if slices.StringInSlice(groups, aclGroup) {
			groupMatch = true
			break
		}
	}

	amMatch := len(acl.Scope.Alertmanagers) == 0
	for _, aclAM := range acl.Scope.Alertmanagers {
		if amName == aclAM {
			amMatch = true
			break
		}
	}

	filterMatch := len(acl.Scope.Filters) == 0
	for _, aclFilter := range acl.Scope.Filters {
		filterMatch = aclFilter.isMatch(silence)
	}

	if groupMatch && amMatch && filterMatch {
		switch acl.Action {
		case aclActionAllow:
			return true, nil
		case aclActionBlock:
			return false, fmt.Errorf("silence blocked by ACL rule: %s", acl.Reason)
		case aclActionRequireMatcher:
			for _, aclM := range acl.Matchers.Required {
				var wasFound bool
				for _, m := range silence.Matchers {
					if m.Name == aclM.Name && m.Value == aclM.Value && m.IsRegex == aclM.IsRegex {
						wasFound = true
						break
					}
				}
				if !wasFound {
					return false, fmt.Errorf("silence blocked by ACL rule: %s", acl.Reason)
				}
			}
		}
	}

	return false, nil
}

func newSilenceACLFromConfig(cfg config.SilenceACLRule) (*silenceACL, error) {
	acl := silenceACL{
		Action: cfg.Action,
		Reason: cfg.Reason,
		Scope: silenceACLScope{
			Groups:        []string{},
			Alertmanagers: []string{},
			Filters:       []silenceFilter{},
		},
		Matchers: aclMatchers{},
	}

	if !slices.StringInSlice(allACLActions, acl.Action) {
		return nil, fmt.Errorf("silence ACL rule requires 'action' to be one of %v, got %q", allACLActions, acl.Action)
	}

	if acl.Reason == "" {
		return nil, fmt.Errorf("silence ACL rule requires 'reason' to be set")
	}

	for _, groupName := range cfg.Scope.Groups {
		var wasFound bool
		for _, authGroup := range config.Config.Authorization.Groups {
			if authGroup.Name == groupName {
				wasFound = true
				break
			}
		}
		if !wasFound {
			return nil, fmt.Errorf("invalid silence ACL rule, no group with name %q found in authorization.groups configuration", groupName)
		}
		acl.Scope.Groups = append(acl.Scope.Groups, groupName)
	}

	for _, amName := range cfg.Scope.Alertmanagers {
		am := alertmanager.GetAlertmanagerByName(amName)
		if am == nil {
			return nil, fmt.Errorf("invalid ACL rule, no alertmanager with name %q found", amName)
		}
		acl.Scope.Alertmanagers = append(acl.Scope.Alertmanagers, am.Name)
	}

	for _, filter := range cfg.Scope.Filters {
		if filter.Name == "" && filter.NameRegex == "" {
			return nil, fmt.Errorf("silence ACL rule filter requires 'name' or 'name_re' to be set")
		}
		if filter.Name != "" && filter.NameRegex != "" {
			return nil, fmt.Errorf("silence ACL rule filter can only have 'name' or 'name_re' set, not both")
		}

		if filter.Value == "" && filter.ValueRegex == "" {
			return nil, fmt.Errorf("silence ACL rule filter requires 'value' or 'value_re' to be set")
		}
		if filter.Value != "" && filter.ValueRegex != "" {
			return nil, fmt.Errorf("silence ACL rule filter can only have 'value' or 'value_re' set, not both")
		}

		f := silenceFilter{
			Name:    filter.Name,
			Value:   filter.Value,
			IsRegex: filter.IsRegex,
		}

		if filter.NameRegex != "" {
			re, err := regexp.Compile(filter.NameRegex)
			if err != nil {
				return nil, fmt.Errorf("invalid ACL rule, failed to parse name_re %q: %s", filter.NameRegex, err)
			}
			f.NameRegex = re
		}

		if filter.ValueRegex != "" {
			re, err := regexp.Compile(filter.ValueRegex)
			if err != nil {
				return nil, fmt.Errorf("invalid ACL rule, failed to parse value_re %q: %s", filter.ValueRegex, err)
			}
			f.ValueRegex = re
		}

		acl.Scope.Filters = append(acl.Scope.Filters, f)
	}

	if acl.Action == aclActionRequireMatcher {
		for _, matcherConfig := range cfg.Matchers.Required {
			if matcherConfig.Name == "" {
				return nil, fmt.Errorf("silence ACL rule matcher requires 'name' to be set")
			}

			if matcherConfig.Value == "" {
				return nil, fmt.Errorf("silence ACL rule matcher requires 'value' to be set")
			}

			m := silenceMatcher{
				Name:    matcherConfig.Name,
				Value:   matcherConfig.Value,
				IsRegex: matcherConfig.IsRegex,
			}

			acl.Matchers.Required = append(acl.Matchers.Required, m)
		}
	}

	return &acl, nil
}
