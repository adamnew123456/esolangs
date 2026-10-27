package main

func buildLoadConstant(mem MemorySpace, start uint32) uint32 {
	hex1 := mem.ReadWord(start)
	hex2 := mem.ReadWord(start + 1)
	if hex1 == '#' && hex2 == '#' {
		 return SCVAddress
	} else if hex1 == '@' && hex2 == '@' {
		 return SCAAddress
	} else if hex1 == '?' && hex2 == '?' {
		 return TSTAddress
	} else if hex1 == '>' && hex2 == '<' {
		 return IPAddress
	} else if hex1 == '<' && hex2 == '>' {
		return IOPAddress
	}

	var value uint32
	if hex1 >= '0' && hex1 <= '9' {
		value = hex1 - '0'
	} else if hex1 >= 'a' && hex1 <= 'f' {
		value = hex1 - 'a' + 0xa
	} else if hex1 >= 'A' && hex1 <= 'F' {
		value = hex1 - 'A' + 0xa
	} else {
		return 0
	}

	value <<= 4
	if hex2 >= '0' && hex2 <= '9' {
		return value | (hex2 - '0')
	} else if hex2 >= 'a' && hex2 <= 'f' {
		return value | (hex2 - 'a' + 0xa)
	} else if hex2 >= 'A' && hex2 <= 'F' {
		return value | (hex2 - 'A' + 0xa)
	} else {
		return 0
	}
}

func IsHalted(mem MemorySpace) bool {
	return mem.ReadWord(HLTAddress) != 0
}

func ExecuteOne(mem MemorySpace) {
	ip := mem.ReadWord(IPAddress)
	instr := mem.ReadWord(ip)
	ip++
	storeIP := true

	if mem.ReadWord(VMFAddress) == 1 {
		if instr == 0 {
			mem.WriteWord(HLTAddress, 1)
		} else if instr == mem.ReadWord(VMTAddress) {
			mem.WriteWord(VMFAddress, 0)
		} else {
			mem.WriteWord(IOPAddress, instr)
		}
	} else {
		var sca, scv uint32
		switch instr {
		case '}':
			mem.WriteWord(VMFAddress, 1)
		case '!':
			mem.WriteWord(SCVAddress, ip)
		case 'X':
			mem.WriteWord(HLTAddress, 1)
		case '#':
			mem.WriteWord(SCVAddress, buildLoadConstant(mem, ip))
			ip += 2
		case '@':
			mem.WriteWord(SCAAddress, buildLoadConstant(mem, ip))
			ip += 2
		case 'l':
			sca = mem.ReadWord(SCAAddress)
			mem.WriteWord(SCVAddress, mem.ReadWord(sca))
		case 's':
			scv = mem.ReadWord(SCVAddress)
			sca = mem.ReadWord(SCAAddress)
			mem.WriteWord(sca, scv)
			storeIP = sca != IPAddress
		case 'm':
			scv = mem.ReadWord(SCVAddress)
			sca = mem.ReadWord(SCAAddress)
			mem.WriteWord(sca, mem.ReadWord(scv))
			storeIP = sca != IPAddress
		case '?':
			if mem.ReadWord(TSTAddress) != 0 {
				scv = mem.ReadWord(SCVAddress)
				sca = mem.ReadWord(SCAAddress)
				mem.WriteWord(sca, mem.ReadWord(scv))
				storeIP = sca != IPAddress
			}
		case '+':
			scv = mem.ReadWord(SCVAddress)
			sca = mem.ReadWord(SCAAddress)
			mem.WriteWord(SCVAddress, scv + sca)
		case '-':
			scv = mem.ReadWord(SCVAddress)
			sca = mem.ReadWord(SCAAddress)
			mem.WriteWord(SCVAddress, scv - sca)
		case '*':
			scv = mem.ReadWord(SCVAddress)
			sca = mem.ReadWord(SCAAddress)
			mem.WriteWord(SCVAddress, scv * sca)
		case '/':
			sca = mem.ReadWord(SCAAddress)
			if sca == 0 {
				mem.WriteWord(SCVAddress, 0)
			} else {
				scv = mem.ReadWord(SCVAddress)
				mem.WriteWord(SCVAddress, scv / sca)
			}
		case '%':
			sca = mem.ReadWord(SCAAddress)
			if sca == 0 {
				mem.WriteWord(SCVAddress, 0)
			} else {
				scv = mem.ReadWord(SCVAddress)
				mem.WriteWord(SCVAddress, scv % sca)
			}
		case '&':
			scv = mem.ReadWord(SCVAddress)
			sca = mem.ReadWord(SCAAddress)
			mem.WriteWord(SCVAddress, scv & sca)
		case '|':
			scv = mem.ReadWord(SCVAddress)
			sca = mem.ReadWord(SCAAddress)
			mem.WriteWord(SCVAddress, scv | sca)
		case 'x':
			scv = mem.ReadWord(SCVAddress)
			sca = mem.ReadWord(SCAAddress)
			mem.WriteWord(SCVAddress, scv ^ sca)
		case '~':
			scv = mem.ReadWord(SCVAddress)
			mem.WriteWord(SCVAddress, ^scv)
		case '>':
			scv = mem.ReadWord(SCVAddress)
			sca = mem.ReadWord(SCAAddress)
			mem.WriteWord(SCVAddress, scv >> sca)
		case '<':
			scv = mem.ReadWord(SCVAddress)
			sca = mem.ReadWord(SCAAddress)
			mem.WriteWord(SCVAddress, scv << sca)
		case '=':
			scv = mem.ReadWord(SCVAddress)
			sca = mem.ReadWord(SCAAddress)
			if scv == sca {
				mem.WriteWord(TSTAddress, 1)
			} else {
				mem.WriteWord(TSTAddress, 0)
			}
		case 'v':
			scv = mem.ReadWord(SCVAddress)
			sca = mem.ReadWord(SCAAddress)
			if scv < sca {
				mem.WriteWord(TSTAddress, 1)
			} else {
				mem.WriteWord(TSTAddress, 0)
			}
		case '^':
			scv = mem.ReadWord(SCVAddress)
			sca = mem.ReadWord(SCAAddress)
			if scv > sca {
				mem.WriteWord(TSTAddress, 1)
			} else {
				mem.WriteWord(TSTAddress, 0)
			}
		}
	}

	if storeIP {
		mem.WriteWord(IPAddress, ip)
	}
}
