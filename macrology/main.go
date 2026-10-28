package main

import (
	"fmt"
	"net"
	"os"
	"strings"
)

func stdinReader() uint32 {
	var inp [1]byte
	_, err := os.Stdin.Read(inp[:])
	if err != nil {
		return 0xffffffff
	} else {
		return uint32(inp[0])
	}
}

func stdoutWriter(value uint32) {
	var outp [1]byte
	outp[0] = byte(value)
	os.Stdout.Write(outp[:])
}

func createDebugger() Debugger {
	config := os.Getenv("MACL_DEBUG")
	if strings.HasPrefix(config, "tcp:") {
		conn, err := net.Dial("tcp", strings.TrimPrefix(config, "tcp:"))
		if err != nil {
			fmt.Printf("Unable to connect debugger to %s: %s\n", config, err.Error())
			return nil
		}
		conn.(*net.TCPConn).SetNoDelay(true)
		return ConnectDebugger(conn)
	} else if strings.HasPrefix(config, "unix:") {
		conn, err := net.Dial("unix", strings.TrimPrefix(config, "unix:"))
		if err != nil {
			fmt.Printf("Unable to connect debugger to %s: %s\n", config, err.Error())
			return nil
		}
		return ConnectDebugger(conn)
	} else {
		if len(config) > 0 {
			fmt.Printf("Invalid MACL_DEBUG: %s\n", config)
		}
		return nil
	}
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: macrology PROGRAM-FILE")
		return
	}

	program, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Printf("Unable to open %s: %s\n", os.Args[1], err.Error())
		return
	}

	mmio := MemoryMappedIO{
		ReadIOPort: stdinReader,
		WriteIOPort: stdoutWriter,
	}
	memory, err := BuildMemorySpace(program, mmio)
	if err != nil {
		fmt.Printf("Error while reading %s: %s\n", os.Args[1], err.Error())
		return
	}

	dbg := createDebugger()
	if dbg != nil {
		memory = dbg.InstrumentMemory(memory)
	}

	for !IsHalted(memory) {
		if dbg != nil {
			if !dbg.StepInstruction() {
				break
			}
			ip := memory.ReadWord(IPAddress)
			instr := memory.ReadWord(ip)
			dbg.TraceInstruction(ip, instr)
		}
		ExecuteOne(memory)
	}
}
