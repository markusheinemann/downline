package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
)

type Set struct {
	fs       *flag.FlagSet
	required []requiredOpt
	errs     []error
}

type requiredOpt struct {
	name string
	set  func() bool
}

func NewSet(name string) *Set {
	return &Set{
		fs: flag.NewFlagSet(name, flag.ContinueOnError),
	}
}

func EnvKey(name string) string {
	return strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
}

func (s *Set) String(p *string, name, def, usage string) {
	if v, ok := os.LookupEnv(EnvKey(name)); ok {
		def = v
	}
	s.fs.StringVar(p, name, def, fmt.Sprintf(" [%s]", usage))
}

func (s *Set) RequiredString(p *string, name, usage string) {
	s.String(p, name, "", fmt.Sprintf(" [%s]", usage))
	s.required = append(s.required, requiredOpt{name, func() bool { return *p != "" }})
}

func (s *Set) Parse(args []string) error {
	if err := s.fs.Parse(args); err != nil {
		return err
	}
	errs := s.errs
	for _, r := range s.required {
		if !r.set() {
			errs = append(errs, fmt.Errorf("\t--%s or %s is required", r.name, EnvKey(r.name)))
		}
	}
	return errors.Join(errs...)
}
