package codeagent

type CodeAgent struct {
	Name string
	Cmd  string
}

func Builtin() []*CodeAgent {
	return []*CodeAgent{
		{
			Name: "Claude Code",
			Cmd:  "claude",
		},
	}
}
