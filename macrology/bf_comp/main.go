package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// Compile brainfuck to Macrology code. Compilation strategy:
//
// 1. The brainfuck memory space goes from 0x100 until 0x80ff, giving a total of
//    0x7fff memory cells. The actual address is stored as an offset from 0x100
//    with the base address added when necessary. < and > mask the offset to
//    keep it within range (enough repeated < or > wrap around).
//
// 2. EOF is represented as 0.
//
// 3. Memory cells go from 0 to 255. As with the cell pointer, memory values are
//    masked to the proper range when written.
//
// 4. Perform only basic repeated-instruction optimizations. Runs of + or - are
//    translated into a single addition/subtraction of the cell value, while
//    runs of < or > are translated into a single addition/subtraction of the
//    cell pointer.
//
// 5. Use [ and ] as debugging indicators. While they do take up code size, they
//    don't do anything and may make the resulting output more readable. It should
//    be easier to trace back individual code fragments to specific functions.
//
// Special addresses:
//
// - cellOffset = 0x20: Memory cell offset
// - 0xfd, 0xfe, 0xff: Temporary storage

const bfMemoryBase = 0x100
const bfMemorySize = 0x7fff
const bfInputBase = 0x10000
const bfCellOffset = 0x20
const bfScratch = 0xfd
const bfScratch2 = 0xfe
const bfScratch3 = 0xff

// The bytes of the constant go into the %02x format strings, from most
// significnat to least significant
const bfLoadScvConst32 = "[#%02x@08< @%02x|@08< @%02x|@08< @%02x|]"

func checkBalancedLoops(program []byte) error {
	depth := 0
	for _, ch := range program {
		switch ch {
		case '[':
			depth++
		case ']':
			depth--
			if depth < 0 {
				return fmt.Errorf("Unbalanced loops detected, aborting")
			}
		}
	}

	if depth != 0 {
		return fmt.Errorf("Unterminated loop detected, aborting")
	}

	return nil
}

type brainfuckLoop struct {
	openAddr int
	bodyAddr int
	afterAddr int
}

type brainfuckCompiler struct {
	input []byte
	inputPos int
	output strings.Builder
	outputPos int
	loopStartAddrStack []int
	loopBounds []brainfuckLoop
}

func initCompiler(input []byte) *brainfuckCompiler {
	comp := &brainfuckCompiler{
		input: input,
		loopStartAddrStack: make([]int, 0, 64),
		loopBounds: make([]brainfuckLoop, 0, 64),
	}
	comp.output.WriteRune('{')
	comp.outputPos++
	return comp
}

func (comp *brainfuckCompiler) hasMore() bool {
	return comp.inputPos < len(comp.input)
}

func (comp *brainfuckCompiler) translateNext() {
	ch := comp.input[comp.inputPos]
	switch ch {
	case '+':
		comp.translatePlus()
	case '-':
		comp.translateMinus()
	case '<':
		comp.translateLeft()
	case '>':
		comp.translateRight()
	case ',':
		comp.translateInput()
	case '.':
		comp.translateOutput()
	case '[':
		comp.translateOpenLoop()
	case ']':
		comp.translateCloseLoop()
	default:
		comp.inputPos++
	}
}

func (comp *brainfuckCompiler) consumeRepeated() int {
	ch := comp.input[comp.inputPos]
	comp.inputPos++

	count := 1
	for comp.hasMore() && comp.input[comp.inputPos] == ch {
		count++
		comp.inputPos++
	}
	return count
}

func (comp *brainfuckCompiler) loadConstantToScv(value uint) {
	var n int
	if value <= 0xff {
		n, _ = fmt.Fprintf(&comp.output, "[#%02x]", value)
	} else if value <= 0xffff {
		hiBits, loBits := value >> 8, value & 0xff
		n, _ = fmt.Fprintf(&comp.output, "[#%02x@08< @%02x|]", hiBits, loBits)
	} else if value <= 0xffffff {
		hiBits, medBits, loBits := value >> 16, (value >> 8) & 0xff, value & 0xff
		n, _ = fmt.Fprintf(&comp.output, "[#%02x@08< @%02x|@08< @%02x|]", hiBits, medBits, loBits)
	} else {
		b1, b2, b3, b4 := value >> 24, (value >> 16) & 0xff, (value >> 8) & 0xff, value & 0xff
		n, _ = fmt.Fprintf(&comp.output, bfLoadScvConst32, b1, b2, b3, b4)
	}
	comp.outputPos += n
}

func (comp *brainfuckCompiler) patchConstantValue(output []byte, constantLoadStartAddr int, newValue uint) {
	newLoad := fmt.Sprintf(bfLoadScvConst32, newValue >> 24, (newValue >> 16) & 0xff, (newValue >> 8) & 0xff, newValue & 0xff)
	copy(output[constantLoadStartAddr:], []byte(newLoad))
}

func (comp *brainfuckCompiler) resolveCellAddrToScratch() {
	// SCV = 0x100
	// SCA = *cellOffset
	// SCV += SCA
	// *scratch = SCV
	comp.loadConstantToScv(bfMemoryBase)
	n, _ := fmt.Fprintf(&comp.output, "[@%02xL+ @%02xs]", bfCellOffset, bfScratch)
	comp.outputPos += n
}

func (comp *brainfuckCompiler) translatePlus() {
	// Downcast to byte here so this wraps to zero after 0xff repeated
	// operations. Incrementing a byte cell 256 times doesn't change it.
	count := byte(comp.consumeRepeated())
	if count == 0 {
		return
	}

	// *scratch = *cellOffset + 0x100
	// SCA = *scratch
	// SCV = *SCA
	// SCV += <count>
	// SCV &= 0xff
	// SCA = *scratch
	// *SCA = SCV
	// SCA = *scratch
	// *SCA = SCV
	comp.resolveCellAddrToScratch()
	n, _ := fmt.Fprintf(&comp.output, "[@%02xLl @%02x+ @ff& @%02xLs]", bfScratch, count, bfScratch)
	comp.outputPos += n
}

func (comp *brainfuckCompiler) translateMinus() {
	count := byte(comp.consumeRepeated())
	if count == 0 {
		return
	}

	// *scratch = *cellOffset + 0x100
	// SCA = *scratch
	// SCV = *SCA
	// SCV -= <count>
	// SCV &= 0xff
	// SCA = *scratch
	// *SCA = SCV
	// SCA = *scratch
	// *SCA = SCV
	comp.resolveCellAddrToScratch()
	n, _ := fmt.Fprintf(&comp.output, "[@%02xLl @%02x- @ff& @%02xLs]", bfScratch, count, bfScratch)
	comp.outputPos += n
}

func (comp *brainfuckCompiler) translateLeft() {
	count := comp.consumeRepeated()
	count &= bfMemorySize
	if count == 0 {
		return
	}

	// SCA = cellOffset
	// SCV = *SCA
	// SCA = <count>
	// SCV -= SCA
	// SCA = cellOffset
	// *SCA = SCV
	// SCV = 0x7fff
	// SCA = *cellOffset
	// SCV &= SCA
	// SCA = cellOffset
	// *SCA = SCV
	n, _ := fmt.Fprintf(&comp.output, "[@%02xl@%02x- @%02xs ", bfCellOffset, count, bfCellOffset)
	comp.outputPos += n
	comp.loadConstantToScv(bfMemorySize)
	n, _ = fmt.Fprintf(&comp.output, " @%02xL& @%02xs]", bfCellOffset, bfCellOffset)
	comp.outputPos += n
}

func (comp *brainfuckCompiler) translateRight() {
	count := comp.consumeRepeated()
	count &= bfMemorySize
	if count == 0 {
		return
	}

	// SCA = cellOffset
	// SCV = *SCA
	// SCA = <count>
	// SCV += SCA
	// SCA = cellOffset
	// *SCA = SCV
	// SCV = 0x7fff
	// SCA = *cellOffset
	// SCV &= SCA
	// SCA = cellOffset
	// *SCA = SCV
	n, _ := fmt.Fprintf(&comp.output, "[@%02xl@%02x+ @%02xs ", bfCellOffset, count, bfCellOffset)
	comp.outputPos += n
	comp.loadConstantToScv(bfMemorySize)
	n, _ = fmt.Fprintf(&comp.output, " @%02xL& @%02xs]", bfCellOffset, bfCellOffset)
	comp.outputPos += n
}

func (comp *brainfuckCompiler) translateInput() {
	// *scratch = *cellOffset + 0x100
	// *scratch3 = 0
	// *scratch2 = IOP
	// SCV = *scratch2
	// SCV = ~SCV
	// SCA = 0
	// TST = SCV = SCA
	// if TST == 1: *scratch2 = *scratch3
	// *scratch = *scratch2
	comp.resolveCellAddrToScratch()
	n, _ := fmt.Fprintf(&comp.output, "[#00@%02xs #<>@%02xm l~@00= #%02x@%02x? #%02x@%02xLm]", bfScratch3, bfScratch2, bfScratch3, bfScratch2, bfScratch2, bfScratch)
	comp.outputPos += n
	comp.inputPos++
}

func (comp *brainfuckCompiler) translateOutput() {
	// *scratch = *cellOffset + 0x100
	// SCA = *scratch
	// SCV = *SCA
	// IOP = SCV
	comp.resolveCellAddrToScratch()
	n, _ := fmt.Fprintf(&comp.output, "[@%02xLl @<>s]", bfScratch)
	comp.outputPos += n
	comp.inputPos++
}

// Loops have a special contract:
//
// - Opening the loop creates a bookmark for the start of loop instruction and
//   saves its address. We generate a template that we can return to and fill in
//   once we know the loop's end address. Its size must not change so that the
//   address of all later loop instructions are correct.
//
// - Closing the loop creates concrete instructions since the loop's start address
//   is known. The loop's start address is moved from the stack to a patch table
//   that maps the start of loop address to the end of loop address.

func (comp *brainfuckCompiler) translateOpenLoop() {
	// *scratch = *cellOffset + 0x100
	// *scratch2 = loopLastInstr
	// SCA = *scratch
	// SCV = *SCA
	// SCA = 0
	// TST = SCA == SCV
	// if TST == 1: IP = *scratch2
	comp.resolveCellAddrToScratch()

	// Must be the biggest constant possible to fit any possible address
	newLoopId := len(comp.loopBounds)
	comp.loopBounds = append(comp.loopBounds, brainfuckLoop{openAddr: comp.outputPos})
	comp.loopStartAddrStack = append(comp.loopStartAddrStack, newLoopId)
	newLoop := &comp.loopBounds[newLoopId]
	comp.loadConstantToScv(0xffffffff)

	n, _ := fmt.Fprintf(&comp.output, "[@%02xs @%02xLl@00= #%02x@><?]", bfScratch2, bfScratch, bfScratch2)
	comp.outputPos += n
	comp.inputPos++
	newLoop.bodyAddr = comp.outputPos
}

func (comp *brainfuckCompiler) translateCloseLoop() {
	idIdx := len(comp.loopStartAddrStack) - 1
	loopId := comp.loopStartAddrStack[idIdx]
	comp.loopStartAddrStack = comp.loopStartAddrStack[:idIdx]
	loop := &comp.loopBounds[loopId]

	// *scratch = *cellOffset + 0x100
	// *scratch2 = loopBodyInstr
	// SCA = *scratch
	// SCV = *SCA
	// SCA = 0
	// TST = SCA > SCV
	// if TST == 1: IP = *scratch2
	comp.resolveCellAddrToScratch()
	comp.loadConstantToScv(uint(loop.bodyAddr) + bfInputBase)
	n, _ := fmt.Fprintf(&comp.output, "[@%02xs @%02xLl @00^ #%02x@><?]", bfScratch2, bfScratch, bfScratch2)
	comp.outputPos += n
	comp.inputPos++
	loop.afterAddr = comp.outputPos
}

func (comp *brainfuckCompiler) finalize() ([]byte, error) {
	if comp.hasMore() {
		return nil, errors.New("Input program not fully processed")
	}

	fmt.Fprint(&comp.output, "[X]")

	output := []byte(comp.output.String())
	for loopId := range comp.loopBounds {
		loop := &comp.loopBounds[loopId]
		comp.patchConstantValue(output, loop.openAddr, uint(loop.afterAddr) + bfInputBase)
	}

	return output, nil
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: bf_comp SOURCE-FILE")
		return
	}

	program, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Printf("Unable to read %s: %s\n", os.Args[1], err.Error())
		return
	}

	err = checkBalancedLoops(program)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	comp := initCompiler(program)
	for comp.hasMore() {
		comp.translateNext()
	}

	result, err := comp.finalize()
	if err != nil {
		fmt.Printf("Error compiling %s: %s\n", os.Args[1], err.Error())
		return
	}
	fmt.Println(string(result))
}
