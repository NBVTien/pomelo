package shell

import "strings"

type Shell interface {
	Name() string
	Login(script string) []string
	Command(script string) []string
	Interactive() []string
}

var Default Shell = Zsh{}

func Login(script string) []string   { return Default.Login(script) }
func Command(script string) []string { return Default.Command(script) }
func Interactive() []string          { return Default.Interactive() }
func Name() string                   { return Default.Name() }

func Quote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

type Zsh struct{}

func (Zsh) Name() string { return "zsh" }

func (Zsh) Login(script string) []string { return []string{"zsh", "-lc", script} }

func (Zsh) Command(script string) []string { return []string{"zsh", "-c", script} }

func (Zsh) Interactive() []string { return []string{"zsh", "-i"} }
