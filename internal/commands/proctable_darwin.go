//go:build darwin

package commands

import "golang.org/x/sys/unix"

type procTable struct {
	name map[int]string
	kids map[int][]int
}

func loadProcTable() *procTable {
	ks, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return nil
	}
	t := &procTable{name: make(map[int]string, len(ks)), kids: make(map[int][]int, len(ks))}
	for i := range ks {
		pid := int(ks[i].Proc.P_pid)
		ppid := int(ks[i].Eproc.Ppid)
		t.name[pid] = cString(ks[i].Proc.P_comm[:])
		t.kids[ppid] = append(t.kids[ppid], pid)
	}
	return t
}

func cString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}
