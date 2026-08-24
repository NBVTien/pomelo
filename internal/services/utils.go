package services

import (
	"fmt"
	"os"
	"strings"
)

type EnvVar struct {
	Key   string
	Value string
}

type GitWorktree struct {
	Path   string
	Branch string
}

type DirMapping struct {
	Name string
	Path string
}

type DirBranch struct {
	Name   string
	Branch string
}

func FileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func DirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func ContainsStr(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func ValidateBranchName(branch string) error {
	if branch == "" {
		return fmt.Errorf("branch name cannot be empty")
	}
	if strings.Contains(branch, "..") {
		return fmt.Errorf("branch name cannot contain '..'")
	}
	if strings.HasPrefix(branch, "/") || strings.HasPrefix(branch, "~") {
		return fmt.Errorf("branch name cannot start with '/' or '~'")
	}
	return nil
}
