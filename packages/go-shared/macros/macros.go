package macros

import (
	"errors"
	"regexp"
	"sort"
	"strings"
)

var macroTokenPattern = regexp.MustCompile(`\{([a-z][a-z0-9_]{0,63})\}`)
var macroNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

func Render(
	template string,
	values map[string]string,
) string {
	out := template
	for k, v := range values {
		out = strings.ReplaceAll(
			out,
			"{"+k+"}",
			v,
		)
	}
	return out
}

func Names(template string) []string {
	matches := macroTokenPattern.FindAllStringSubmatch(
		template,
		-1,
	)
	if len(matches) == 0 {
		return nil
	}

	seen := make(
		map[string]struct{},
		len(matches),
	)
	for _, match := range matches {
		if len(match) != 2 {
			continue
		}
		seen[match[1]] = struct{}{}
	}

	names := make(
		[]string,
		0,
		len(seen),
	)
	for name := range seen {
		names = append(
			names,
			name,
		)
	}
	sort.Strings(names)
	return names
}

func ValidateName(name string) error {
	if !macroNamePattern.MatchString(name) {
		return errors.New("invalid macro name")
	}
	return nil
}

func ValidateTemplate(
	template string,
	allowed map[string]struct{},
) error {
	for _, name := range Names(template) {
		if _, ok := allowed[name]; !ok {
			return errors.New("unknown macro: " + name)
		}
	}
	return nil
}
