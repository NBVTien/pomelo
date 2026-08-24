package commands

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/process"
	"github.com/pomelohq/pomelo/internal/ptyhost"
	"golang.org/x/term"
)

type procRow struct {
	PID   int
	Kind  string
	Label string
}

func gatherProcs() []procRow {
	var rows []procRow
	for _, h := range ptyhost.Holders() {
		rows = append(rows, procRow{PID: h.PID, Kind: "pty", Label: h.Name})
	}
	return rows
}

type psNode struct {
	pid   int
	depth int
	kind  string
	label string
	cpu   float64
	cpuOK bool
	ram   float64
}

func flatten(tbl *procTable, pid, depth int, kind, label string, out *[]psNode) {
	*out = append(*out, psNode{pid: pid, depth: depth, kind: kind, label: label})
	if tbl == nil {
		return
	}
	kids := append([]int(nil), tbl.kids[pid]...)
	sort.Ints(kids)
	for _, k := range kids {
		flatten(tbl, k, depth+1, "", tbl.name[k], out)
	}
}

type psSampler struct {
	prev map[int]*process.Process
}

func newSampler() *psSampler { return &psSampler{prev: map[int]*process.Process{}} }

func (s *psSampler) sample() []psNode {
	tbl := loadProcTable()
	var nodes []psNode
	for _, r := range gatherProcs() {
		flatten(tbl, r.PID, 0, r.kindOf(), r.Label, &nodes)
	}
	cur := map[int]*process.Process{}
	for i := range nodes {
		n := &nodes[i]
		p := cur[n.pid]
		if p == nil {
			p = s.prev[n.pid]
			if p == nil {
				np, err := process.NewProcess(int32(n.pid))
				if err != nil {
					continue
				}
				p = np
				_, _ = p.Percent(0)
			}
			cur[n.pid] = p
		}
		if c, err := p.Percent(0); err == nil {
			n.cpu, n.cpuOK = c, true
		}
		if fp := physFootprintBytes(n.pid); fp > 0 {
			n.ram = float64(fp) / 1e6
		} else if mi, err := p.MemoryInfo(); err == nil && mi != nil {
			n.ram = float64(mi.RSS) / 1e6
		}
	}
	s.prev = cur
	return nodes
}

func (r procRow) kindOf() string { return r.Kind }

type ResourceStat struct {
	Procs int     `json:"procs"`
	CPU   float64 `json:"cpu"`
	RAMMB float64 `json:"ram_mb"`
}

type ResourceSampler struct{ s *psSampler }

func NewResourceSampler() *ResourceSampler { return &ResourceSampler{s: newSampler()} }

func (r *ResourceSampler) Sample() ResourceStat {
	nodes := r.s.sample()
	st := ResourceStat{Procs: len(nodes)}
	for _, n := range nodes {
		st.CPU += n.cpu
		st.RAMMB += n.ram
	}
	return st
}

type ProcRow struct {
	PID   int     `json:"pid"`
	Kind  string  `json:"kind"`
	Label string  `json:"label"`
	CPU   float64 `json:"cpu"`
	RAMMB float64 `json:"ram_mb"`
}

func (r *ResourceSampler) SampleByHolder() []ProcRow {
	nodes := r.s.sample()
	var rows []ProcRow
	for _, n := range nodes {
		if n.depth == 0 {
			rows = append(rows, ProcRow{PID: n.pid, Kind: n.kind, Label: n.label, CPU: n.cpu, RAMMB: n.ram})
		} else if len(rows) > 0 {
			rows[len(rows)-1].CPU += n.cpu
			rows[len(rows)-1].RAMMB += n.ram
		}
	}
	return rows
}

func Ps(watch bool) {
	sampler := newSampler()
	if !watch {
		sampler.sample()
		time.Sleep(700 * time.Millisecond)
		printStatic(sampler.sample())
		return
	}
	runTUI(sampler)
}

func printStatic(nodes []psNode) {
	fmt.Printf("%s%-7s %-5s %8s %9s  %s%s\n", Bold, "PID", "KIND", "CPU%", "RAM(MB)", "WHAT", NC)
	if len(nodes) == 0 {
		fmt.Printf("  %sno pom-spawned processes%s\n", Dim, NC)
		return
	}
	for _, n := range nodes {
		cpu := "-"
		col := ""
		if n.cpuOK {
			cpu = fmt.Sprintf("%.1f", n.cpu)
			col = cpuColor(n.cpu)
		}
		fmt.Printf("%-7d %-5s %s%8s%s %9.1f  %s\n", n.pid, n.kind, col, cpu, NC, n.ram, treeLabel(n))
	}
}

func treeLabel(n psNode) string {
	if n.depth == 0 {
		return n.label
	}
	return strings.Repeat("  ", n.depth) + "└ " + n.label
}

func cpuColor(c float64) string {
	switch {
	case c >= 80:
		return Red
	case c >= 30:
		return Yellow
	default:
		return Green
	}
}

func cpuBar(c float64, width int) string {
	if c < 0 {
		c = 0
	}
	if c > 100 {
		c = 100
	}
	fill := int(c/100*float64(width) + 0.5)
	return cpuColor(c) + strings.Repeat("█", fill) + Dim + strings.Repeat("·", width-fill) + NC
}

const (
	altScreenOn  = "\033[?1049h\033[?25l"
	altScreenOff = "\033[?25h\033[?1049l"
	cursorHome   = "\033[H"
	clearBelow   = "\033[J"
	clearEOL     = "\033[K"
)

func runTUI(sampler *psSampler) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) || !term.IsTerminal(int(os.Stdout.Fd())) {
		for {
			fmt.Print("\033[H\033[2J")
			printStatic(sampler.sample())
			time.Sleep(time.Second)
		}
	}

	old, err := term.MakeRaw(fd)
	if err != nil {
		printStatic(sampler.sample())
		return
	}
	restore := func() {
		fmt.Print(altScreenOff)
		_ = term.Restore(fd, old)
	}
	defer restore()

	quit := make(chan struct{})
	go func() {
		buf := make([]byte, 8)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				close(quit)
				return
			}
			for _, b := range buf[:n] {
				if b == 'q' || b == 'Q' || b == 0x03 || b == 0x1b {
					close(quit)
					return
				}
			}
		}
	}()

	fmt.Print(altScreenOn)
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	render(sampler.sample())
	for {
		select {
		case <-quit:
			return
		case <-tick.C:
			render(sampler.sample())
		}
	}
}

func render(nodes []psNode) {
	cols, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || cols <= 0 {
		cols = 100
	}
	var totCPU, totRAM float64
	for _, n := range nodes {
		if n.cpuOK {
			totCPU += n.cpu
		}
		totRAM += n.ram
	}

	var b strings.Builder
	b.WriteString(cursorHome)
	fmt.Fprintf(&b, "%spom ps%s  %s%d procs · %.0f%% CPU · %.0f MB%s   %sq to quit%s%s\r\n",
		Bold, NC, Cyan, len(nodes), totCPU, totRAM, NC, Dim, NC, clearEOL)
	b.WriteString("\r\n" + clearEOL)
	fmt.Fprintf(&b, "%s%-7s %-5s %6s %-12s %8s  %s%s\r\n",
		Bold, "PID", "KIND", "CPU%", "", "RAM(MB)", "WHAT", NC+clearEOL)

	if len(nodes) == 0 {
		b.WriteString("  " + Dim + "no pom-spawned processes" + NC + clearEOL + "\r\n")
		b.WriteString(clearBelow)
		fmt.Print(b.String())
		return
	}

	const fixed = 7 + 1 + 5 + 1 + 6 + 1 + 12 + 1 + 8 + 2
	whatMax := cols - fixed
	if whatMax < 8 {
		whatMax = 8
	}
	for _, n := range nodes {
		cpu, col, bar := "-", "", strings.Repeat(" ", 12)
		if n.cpuOK {
			cpu = fmt.Sprintf("%.1f", n.cpu)
			col = cpuColor(n.cpu)
			bar = cpuBar(n.cpu, 12)
		}
		label := truncate(treeLabel(n), whatMax)
		fmt.Fprintf(&b, "%-7d %s%-5s%s %s%6s%s %s %8.1f  %s%s\r\n",
			n.pid, Dim, n.kind, NC, col, cpu, NC, bar, n.ram, label, clearEOL)
	}
	b.WriteString(clearBelow)
	fmt.Print(b.String())
}

func truncate(s string, max int) string {
	if len([]rune(s)) <= max {
		return s
	}
	r := []rune(s)
	if max <= 1 {
		return string(r[:max])
	}
	return string(r[:max-1]) + "…"
}
