package main

import (
	"fmt"
	"os"
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

	for !IsHalted(memory) {
		ExecuteOne(memory)
	}
}
