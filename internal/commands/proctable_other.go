//go:build !darwin

package commands

type procTable struct {
	name map[int]string
	kids map[int][]int
}

func loadProcTable() *procTable { return nil }
