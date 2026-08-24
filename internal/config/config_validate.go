package config

import (
	"fmt"
	"sort"
	"strings"
)

func (c *Config) Validate() error {
	var errs []string

	for _, dirName := range c.RepoOrder {
		dir := c.Repos[dirName]
		if dir == nil {
			continue
		}
		ctx := fmt.Sprintf("repo %q", dirName)
		scan := func(where string, vals map[string]string) {
			for k, val := range vals {
				for _, ref := range templateRefs(val) {
					c.checkRef(&errs, fmt.Sprintf("%s %s.%s", ctx, where, k), ref)
				}
			}
		}
		scan("env", dir.Env)
		for _, entry := range dir.EnvOutput {
			scan(entry.File, entry.Env)
		}
		for _, svcName := range dir.ServiceOrder {
			if svc := dir.Services[svcName]; svc != nil {
				scan("service:"+svcName, svc.Env)
			}
		}

		c.checkProfiles(&errs, ctx, dir.Profiles)
		for _, svcName := range dir.ServiceOrder {
			if svc := dir.Services[svcName]; svc != nil {
				c.checkProfiles(&errs, ctx+" service "+svcName, svc.Profiles)
			}
		}
	}

	if len(errs) == 0 {
		return nil
	}
	sort.Strings(errs)
	return fmt.Errorf("invalid pom.yml:\n  - %s", strings.Join(errs, "\n  - "))
}

func (c *Config) checkProfiles(errs *[]string, ctx string, profiles StringList) {
	for _, p := range profiles {
		if p == "local" {
			continue
		}
		if _, ok := c.Environments[p]; !ok {
			*errs = append(*errs, fmt.Sprintf("%s: environment %q not defined", ctx, p))
		}
	}
}

type templateRef struct{ ns, name string }

func templateRefs(s string) []templateRef {
	var out []templateRef
	for {
		start := strings.Index(s, "{{")
		if start < 0 {
			break
		}
		end := strings.Index(s[start:], "}}")
		if end < 0 {
			break
		}
		inner := strings.TrimSpace(s[start+2 : start+end])
		s = s[start+end+2:]
		ns, name, ok := strings.Cut(inner, ":")
		if !ok {
			continue
		}
		if bar := strings.IndexByte(name, '|'); bar >= 0 {
			name = name[:bar]
		}
		out = append(out, templateRef{ns: strings.TrimSpace(ns), name: strings.TrimSpace(name)})
	}
	return out
}

var colonReplacement = map[string]string{
	"conn": "{{shared.NAME.url}}",
	"host": "{{shared.NAME.host}}",
	"port": "{{shared.NAME.port}} (or {{<repo>.<svc>.port}})",
	"user": "{{shared.NAME.user}}",
	"pass": "{{shared.NAME.pass}}",
	"slot": "{{shared.NAME.slot}} (or {{slot.NAME}})",
	"db":   "{{db.NAME}}",
	"var":  "",
	"url":  "",
	"ws":   "",
}

func (c *Config) checkRef(errs *[]string, ctx string, ref templateRef) {
	repl, known := colonReplacement[ref.ns]
	if !known {
		*errs = append(*errs, fmt.Sprintf("%s: {{%s:%s}} — colon-form templates are removed; use dot notation", ctx, ref.ns, ref.name))
		return
	}
	if repl == "" {
		*errs = append(*errs, fmt.Sprintf("%s: {{%s:%s}} is removed with no replacement — use a dot-notation ref (e.g. {{<repo>.<svc>.url}})", ctx, ref.ns, ref.name))
		return
	}
	*errs = append(*errs, fmt.Sprintf("%s: {{%s:%s}} is removed — use %s", ctx, ref.ns, ref.name, repl))
}
