package sysenv

import (
	"encoding"
	"errors"
	"fmt"
	"strings"
)

// An Environment represents a deployment environment for a system.
type Environment int16

const (
	// Unknown environment type.
	Unknown Environment = iota
	// Local environment for development and testing on a developer's machine.
	Local
	// Development environment for engineers to test using a remote infrastructure which mirrors Live,
	// aiming to be as close as possible to real-world scenarios.
	Development
	// Sandbox environment for engineers to test using a remote infrastructure which mirrors Live,
	// aiming to be as close as possible to real-world scenarios.
	Sandbox
	// Demo environment for stakeholders to test and approve before going live.
	Demo
	// Live environment for general use by end-users. Also known as production.
	Production
)

var (
	_ fmt.Stringer             = (*Environment)(nil)
	_ encoding.TextUnmarshaler = (*Environment)(nil)
	_ encoding.TextMarshaler   = (*Environment)(nil)

	_toStringMap = map[Environment]string{
		Unknown:     "unknown",
		Local:       "local",
		Development: "development",
		Sandbox:     "sandbox",
		Demo:        "demo",
		Production:  "production",
	}
	_fromStringMap = map[string]Environment{
		"unknown":     Unknown,
		"local":       Local,
		"development": Development,
		"develop":     Development, // alias
		"dev":         Development, // alias
		"sandbox":     Sandbox,
		"snx":         Sandbox, // alias
		"demo":        Demo,
		"production":  Production,
		"prod":        Production, // alias
	}
	_toShortStringMap = map[Environment]string{
		Unknown:     "",
		Local:       "local",
		Development: "dev",
		Sandbox:     "snx",
		Demo:        "demo",
		Production:  "prod",
	}
)

// Parse parses a string into an Environment. If the string does not match any known environment,
// Unknown is returned.
func Parse(v string) Environment {
	out, ok := _fromStringMap[strings.ToLower(v)]
	if !ok {
		return Unknown
	}
	return out
}

// String returns a string representation of the environment.
func (e Environment) String() string {
	return _toStringMap[e]
}

// ShortString returns a short string representation of the environment.
func (e Environment) ShortString() string {
	return _toShortStringMap[e]
}

func (e Environment) MarshalText() (text []byte, err error) {
	return []byte(e.String()), nil
}

func (e *Environment) UnmarshalText(text []byte) error {
	*e = Parse(string(text))
	if *e == Unknown {
		return errors.New("unknown environment")
	}
	return nil
}
