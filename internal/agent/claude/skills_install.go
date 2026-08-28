package claude

import (
	_ "embed"
	"os"
	"path/filepath"
)

//go:embed skills/pom-review/SKILL.md
var pomReviewSkill string

// pomReviewSkillPath is where the bundled pom-review skill is written so Claude
// (terminal + in-app agent) can load it from the user's global skills dir.
func pomReviewSkillPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "skills", "pom-review", "SKILL.md"), nil
}

// InstallGlobalSkills writes the app-bundled skills into ~/.claude/skills so the
// agent can author reviews. App-managed: overwritten each launch to track the app.
func InstallGlobalSkills() error {
	path, err := pomReviewSkillPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(pomReviewSkill), 0o644)
}

// SkillsInstalled reports whether the bundled skill is present and current.
func SkillsInstalled() (installed bool, current bool, path string) {
	p, err := pomReviewSkillPath()
	if err != nil {
		return false, false, ""
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return false, false, p
	}
	return true, string(b) == pomReviewSkill, p
}
