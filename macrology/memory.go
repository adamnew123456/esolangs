package main

import (
	"errors"
	"io"
)

type MemorySpace interface {
	ReadWord(addr uint32) uint32
	WriteWord(addr, value uint32)
}

type MemoryMappedIO struct {
	ReadIOPort func() uint32
	WriteIOPort func(value uint32)
}

type DirectMemorySpace struct {
	userSystem []uint32
	inputStream []uint32
	MemoryMappedIO
}

const InputStreamBase uint32 = 0x10000
const InputStreamMaxLength uint32 = 0xffffffff - InputStreamBase + 1

const (
	VMFAddress uint32 = 0
	VMTAddress uint32 = 1
	HLTAddress uint32 = 2
	SCVAddress uint32 = 3
	SCAAddress uint32 = 4
	TSTAddress uint32 = 5
	EIPAddress uint32 = 6
	IPAddress uint32 = 7
	IOPAddress uint32 = 8
)

func BuildMemorySpace(program io.Reader, mmio MemoryMappedIO) (MemorySpace, error) {
	pages := make([][]byte, 0, 8)
	var programSize uint32
	eof := false
	for !eof && programSize <= InputStreamMaxLength {
		page := make([]byte, 4096)
		n, err := program.Read(page)
		eof = errors.Is(err, io.EOF)
		if err != nil && !eof {
			return nil, err
		}
		programSize += uint32(n)
		pages = append(pages, page[:n])
	}

	inputStream := make([]uint32, min(programSize, InputStreamMaxLength))
	offset := 0
	for _, page := range pages {
		for i := range page {
			if offset >= int(InputStreamMaxLength) {
				break
			}
			inputStream[offset] = uint32(page[i])
			offset++
		}
	}

	userSystem := make([]uint32, 0xffff)
	userSystem[VMFAddress] = 1
	userSystem[VMTAddress] = '{'
	userSystem[EIPAddress] = InputStreamBase + uint32(programSize)
	userSystem[IPAddress] = InputStreamBase

	return &DirectMemorySpace{
		userSystem: userSystem,
		inputStream: inputStream,
		MemoryMappedIO: mmio,
	}, nil
}

func (mem *DirectMemorySpace) ReadWord(addr uint32) uint32 {
	if addr == IOPAddress {
		return mem.ReadIOPort()
	}
	if addr < InputStreamBase {
		return mem.userSystem[addr]
	}

	inpAddr := addr - InputStreamBase
	if inpAddr < uint32(len(mem.inputStream)) {
		return uint32(mem.inputStream[inpAddr])
	}
	return 0
}

func (mem *DirectMemorySpace) WriteWord(addr, value uint32) {
	if addr == EIPAddress {
		return
	}

	if addr == IOPAddress {
		mem.WriteIOPort(value)
	} else if addr < InputStreamBase {
		mem.userSystem[addr] = value
	}
}
