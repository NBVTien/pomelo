package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/pomelohq/pomelo/internal/ptyhost"
	"golang.org/x/term"
)

var ptyCmd = &cobra.Command{
	Use:   "pty",
	Short: "Experimental self-managed PTY sessions (tmux-free) — run / attach",
}

var (
	ptyRunCwd  string
	ptyRunCols int
	ptyRunRows int
)

var ptyRunCmd = &cobra.Command{
	Use:   "run <name> -- <cmd> [args...]",
	Short: "Host a command on a PTY behind a socket (detach-survivable)",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, argv := args[0], args[1:]
		cwd := ptyRunCwd
		if cwd == "" {
			cwd, _ = os.Getwd()
		}
		s, ln, err := ptyhost.ListenAndServe(name, ptyhost.StartOpts{
			Argv: argv, Dir: cwd, Env: os.Environ(), Cols: ptyRunCols, Rows: ptyRunRows,
		})
		if err != nil {
			return err
		}
		defer ln.Close()
		fmt.Printf("[pom] serving %q on %s — `pom pty attach %s` to connect\n", name, ptyhost.SocketPath(name), name)
		return s.Wait()
	},
}

var ptyAttachCmd = &cobra.Command{
	Use:   "attach <name>",
	Short: "Attach a terminal to a running pty session (Ctrl-\\ to detach)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := net.Dial("unix", ptyhost.SocketPath(args[0]))
		if err != nil {
			return fmt.Errorf("no session %q (is it running?): %w", args[0], err)
		}
		defer conn.Close()

		fd := int(os.Stdin.Fd())
		old, err := term.MakeRaw(fd)
		if err != nil {
			return err
		}
		defer term.Restore(fd, old)

		resize := make(chan [2]int, 1)
		sendSize := func() {
			if w, h, err := term.GetSize(fd); err == nil {
				select {
				case resize <- [2]int{w, h}:
				default:
				}
			}
		}
		sendSize()
		winch := make(chan os.Signal, 1)
		signal.Notify(winch, syscall.SIGWINCH)
		defer signal.Stop(winch)
		go func() {
			for range winch {
				sendSize()
			}
		}()

		fmt.Fprint(os.Stdout, "\033[2J\033[3J\033[H")
		fmt.Fprint(os.Stderr, "[pom] attached — press Ctrl-\\ to detach\r\n")
		err = ptyhost.Attach(conn, os.Stdin, os.Stdout, resize, 0x1c)
		fmt.Fprint(os.Stdout, "\033[2J\033[3J\033[H")
		fmt.Fprint(os.Stderr, "[pom] detached\r\n")
		return err
	},
}

var ptyKillCmd = &cobra.Command{
	Use:   "kill <name>",
	Short: "Kill a pty session (e.g. a runaway holder)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ptyhost.KillHolder(args[0]); err != nil {
			return err
		}
		fmt.Printf("Killed %q.\n", args[0])
		return nil
	},
}

func init() {
	ptyRunCmd.Flags().StringVar(&ptyRunCwd, "cwd", "", "working directory for the session")
	ptyRunCmd.Flags().IntVar(&ptyRunCols, "cols", 0, "initial terminal columns")
	ptyRunCmd.Flags().IntVar(&ptyRunRows, "rows", 0, "initial terminal rows")
	ptyCmd.AddCommand(ptyRunCmd, ptyAttachCmd, ptyKillCmd)
}
