package main

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
)

const (
	SingleStep = iota
	RunToTrap
	RunToEnd
)

type Debugger interface {
	StepInstruction() bool
	TraceInstruction(addr, instr uint32)
	InstrumentMemory(mem MemorySpace) MemorySpace
}

type Traps struct {
	addrs map[uint32]bool
	wildcard bool
}

type SocketDebugger struct {
	client     net.Conn
	memory     MemorySpace
	runMode    int
	readTraps, writeTraps Traps
}

func ConnectDebugger(conn net.Conn) *SocketDebugger {
	fmt.Fprintln(conn, "-*- Macrology Debugger -*-")
	return &SocketDebugger{
		client:     conn,
		runMode:    SingleStep,
		readTraps:  Traps{addrs: make(map[uint32]bool)},
		writeTraps: Traps{addrs: make(map[uint32]bool)},
	}
}

func (dbg *SocketDebugger) StepInstruction() bool {
	if dbg.client == nil || dbg.runMode != SingleStep {
		return true
	}

	rdr := bufio.NewReader(dbg.client)
	var inputDone bool
	for !inputDone {
		cmdline, err := rdr.ReadString('\n')
		if err != nil {
			dbg.client.Close()
			dbg.client = nil
			break
		}

		cmdline = strings.TrimSuffix(cmdline, "\n")
		args := strings.Split(cmdline, " ")

		switch args[0] {
		case "k":
			fmt.Fprintln(dbg.client, "Exiting...")
			return false
		case "c":
			dbg.runMode = RunToTrap
			fmt.Fprintln(dbg.client, "Continuing to exit or trap...")
			inputDone = true
		case "C":
			dbg.runMode = RunToEnd
			fmt.Fprintln(dbg.client, "Continuing to exit...")
			inputDone = true
		case "s":
			fmt.Fprintln(dbg.client, "Stepping")
			inputDone = true
		case "r":
			if len(args) != 2 {
				fmt.Fprintln(dbg.client, "Usage: r HEX-ADDR")
			} else {
				addr, err := strconv.ParseUint(args[1], 16, 32)
				if err != nil {
					fmt.Fprintf(dbg.client, "Address error: %s\n", err.Error())
				} else {
					value := dbg.memory.ReadWord(uint32(addr))
					if value >= ' ' && value <= '~' {
						fmt.Fprintf(dbg.client, "%08x -> %08x (%c)\n", addr, value, value)
					} else {
						fmt.Fprintf(dbg.client, "%08x -> %08x\n", addr, value)
					}
				}
			}
		case "w":
			if len(args) != 3 {
				fmt.Fprintln(dbg.client, "Usage: w HEX-ADDR HEX-VALUE")
			} else {
				addr, err := strconv.ParseUint(args[1], 16, 32)
				if err != nil {
					fmt.Fprintf(dbg.client, "Address error: %s\n", err.Error())
				} else {
					value, err := strconv.ParseUint(args[2], 16, 32)
					if err != nil {
						fmt.Fprintf(dbg.client, "Address error: %s\n", err.Error())
					} else {
						dbg.memory.WriteWord(uint32(addr), uint32(value))
						if value >= ' ' && value <= '~' {
							fmt.Fprintf(dbg.client, "%08x <- %08x (%c)\n", addr, value, value)
						} else {
							fmt.Fprintf(dbg.client, "%08x <- %08x\n", addr, value)
						}
					}
				}
			}
		case "t":
			if len(args) != 3 {
				fmt.Fprintln(dbg.client, "Usage: t (r|w) (HEX-ADDR|*)")
			} else {
				var target *Traps
				if args[1] == "r" {
					target = &dbg.readTraps
				} else if args[1] == "w" {
					target = &dbg.writeTraps
				} else {
					fmt.Fprintf(dbg.client, "%s not a valid trap mode", args[2])
				}
				if target != nil {
					if args[2] == "*" {
						target.wildcard = true
					} else {
						addr, err := strconv.ParseUint(args[2], 16, 32)
						if err != nil {
							fmt.Fprintf(dbg.client, "Address error: %s\n", err.Error())
						} else {
							target.addrs[uint32(addr)] = true
						}
					}
				}
			}
		case "l":
			fmt.Fprintln(dbg.client, "Read traps")
			if dbg.readTraps.wildcard {
				fmt.Fprintln(dbg.client, "  Wildcard trap enabled")
			}
			for read := range dbg.readTraps.addrs {
				fmt.Fprintf(dbg.client, "  %08x\n", read)
			}
			fmt.Fprintln(dbg.client, "Write traps")
			if dbg.writeTraps.wildcard {
				fmt.Fprintln(dbg.client, "  Wildcard trap enabled")
			}
			for write := range dbg.writeTraps.addrs {
				fmt.Fprintf(dbg.client, "  %08x\n", write)
			}
		case "d":
			if len(args) != 2 {
				fmt.Fprintln(dbg.client, "Usage: d HEX-ADDR")
			} else {
				if args[1] == "*" {
					dbg.readTraps.wildcard = false
					dbg.writeTraps.wildcard = false
				} else {
					addr, err := strconv.ParseUint(args[1], 16, 32)
					if err != nil {
						fmt.Fprintf(dbg.client, "Address error: %s\n", err.Error())
					} else {
						delete(dbg.readTraps.addrs, uint32(addr))
						delete(dbg.writeTraps.addrs, uint32(addr))
						fmt.Fprintln(dbg.client, "OK")
					}
				}
			}
		case "g":
			fmt.Fprintf(dbg.client, "[%02x] VMF = %08x\n", VMFAddress, dbg.memory.ReadWord(VMFAddress))
			fmt.Fprintf(dbg.client, "[%02x] VMT = %08x\n", VMTAddress, dbg.memory.ReadWord(VMTAddress))
			fmt.Fprintf(dbg.client, "[%02x] HLT = %08x\n", HLTAddress, dbg.memory.ReadWord(HLTAddress))
			fmt.Fprintf(dbg.client, "[%02x] SCV = %08x\n", SCVAddress, dbg.memory.ReadWord(SCVAddress))
			fmt.Fprintf(dbg.client, "[%02x] SCA = %08x\n", SCAAddress, dbg.memory.ReadWord(SCAAddress))
			fmt.Fprintf(dbg.client, "[%02x] TST = %08x\n", TSTAddress, dbg.memory.ReadWord(TSTAddress))
			fmt.Fprintf(dbg.client, "[%02x] EIP = %08x\n", EIPAddress, dbg.memory.ReadWord(EIPAddress))
			fmt.Fprintf(dbg.client, "[%02x] IP = %08x\n", IPAddress, dbg.memory.ReadWord(IPAddress))
		default:
			fmt.Fprintf(dbg.client, `Bad commandline: '%s'
Commands:
  kill process: k
  continue to trap: c
  continue to end: C
  step: s
  read: r HEX-ADDR
  write: w HEX-ADDR HEX-VALUE
  read trap: t r HEX-ADDR|*
  write trap: t w HEX-ADDR|*
  list traps: l
  delete trap: d HEX-ADDR|*
  view registers: g
`, cmdline)
		}
	}
	return true
}

func (dbg *SocketDebugger) TraceInstruction(addr, instr uint32) {
	if dbg.client == nil {
		return
	}

	if instr >= ' ' && instr <= '~' {
		fmt.Fprintf(dbg.client, "exec [%08x] %08x %c\n", addr, instr, instr)
	} else {
		fmt.Fprintf(dbg.client, "exec [%08x] %08x\n", addr, instr)
	}
}

func (dbg *SocketDebugger) InstrumentMemory(mem MemorySpace) MemorySpace {
	dbg.memory = mem
	return dbg
}

func (dbg *SocketDebugger) ReadWord(addr uint32) uint32 {
	value := dbg.memory.ReadWord(addr)
	if dbg.client != nil && (dbg.readTraps.wildcard || dbg.readTraps.addrs[addr]) {
		if dbg.runMode == RunToTrap {
			dbg.runMode = SingleStep
		}
		fmt.Fprintf(dbg.client, "read addr=%08x value=%08x\n", addr, value)
	}
	return value
}

func (dbg *SocketDebugger) WriteWord(addr, value uint32) {
	if dbg.client != nil && (dbg.writeTraps.wildcard || dbg.writeTraps.addrs[addr]) {
		if dbg.runMode == RunToTrap {
			dbg.runMode = SingleStep
		}
		fmt.Fprintf(dbg.client, "write addr=%08x value=%08x\n", addr, value)
	}
	dbg.memory.WriteWord(addr, value)
}
