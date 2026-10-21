//go:build desktop && !windows

package main

import "syscall"

func processAlive(pid int) bool { return syscall.Kill(pid, 0) == nil }
