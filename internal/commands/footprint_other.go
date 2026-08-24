//go:build !darwin

package commands

func physFootprintBytes(pid int) uint64 { return 0 }
