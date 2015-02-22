package main

import (
	"fmt"
	"math/rand"
	"os"
	"bufio"
	"time"
)

/*
Memory Map
0x000-0x1FF: Interpreter
0x050-0x0A0: 4x5 font set
0x200-0xFFF: RAM and program(ROM)
*/


var opcode	uint16		//35 16bit opcodes
var memory	[4096]uint8	//4KB of memory
var V		[16]uint8	//16 bytes for 15 registers + 1 register used as 'carry flag'
var I		uint16		//index register
var pc		uint16		//program counter
var gfx		[64 * 32]uint8	//graphics display
var delayTimer	uint8		//will count down to 0 (60Hz)
var soundTimer	uint8		//will sound buzzer when at 0
var stack	[16]uint16	//used to remember current location before jumps
var sp		uint16		//stack pointer
var key		[16]uint8	//keypad states
var drawFlag	bool

func cycle(){
	opcode = uint16(memory[pc]) << 8 | uint16(memory[pc + 1])

	//fmt.Printf("OPCODE: 0x%d\n", opcode)
	switch opcode & 0xF000{
	case 0x0000:
		switch opcode & 0x000F{
		case 0x0000:	//0x00E0: Clears screen
			clearScreen()
			goToNextInstruction()
		case 0x000E:	//0x00EE: Returns from subroutine
			sp--
			pc = stack[sp]
			goToNextInstruction()
		default:
			fmt.Printf("Unknown or unsupported opcode: 0x%d\n", opcode)
		}
	case 0x1000:		//0x1NNN: Jumps to address NNN
		jump(opcode & 0x0FFF)
	case 0x2000:		//0x2NNN: Calls subroutine at NNN
		call(opcode & 0x0FFF)
	case 0x3000:		//0x3XNN: Skips the next instruction if VX equals NN
		x := (opcode & 0x0F00) >> 8
		nn := opcode & 0x00FF
		if uint16(V[x]) == nn{
			skipNextInstruction()
		}else{
			goToNextInstruction()
		}
	case 0x4000:		//0x4XNN: Skips the next instruction if VX is not equal to NN
		x := (opcode & 0x0F00) >> 8
		nn := opcode & 0x00FF
		if uint16(V[x]) != nn{
			skipNextInstruction()
		}else{
			goToNextInstruction()
		}
	case 0x5000:		//0x5XY0: Skips the next instruction if VX equals VY
		x := (opcode & 0x0F00) >> 8
		y := opcode & 0x00F0
		if V[x] == V[y]{
			skipNextInstruction()
		}else{
			goToNextInstruction()
		}
	case 0x6000:		//0x6XNN: Sets VX to NN
		x := (opcode & 0x0F00) >> 8
		nn := opcode & 0x00FF
		V[x] = uint8(nn)
		goToNextInstruction()
	case 0x7000:		//0x7XNN: Adds NN to VX
		x := (opcode & 0x0F00) >> 8
		nn := opcode & 0x00FF
		V[x] += uint8(nn)
		goToNextInstruction()
	case 0x8000:
		switch opcode & 0x000F{
		case 0x0000:	//0x8XY0: Sets VX to the value of VY
			x := (opcode & 0x0F00) >> 8
			y := (opcode & 0x00F0) >> 4
			V[x] = V[y]
			goToNextInstruction()
		case 0x0001:	//0x8XY1: Sets VX to VX or VY
			x := (opcode & 0x0F00) >> 8
			y := (opcode & 0x00F0) >> 4
			V[x] = V[x] | V[y]
			goToNextInstruction()
		case 0x0002:	//0x8XY2: Sets VX to VX and VY
			x := (opcode & 0x0F00) >> 8
			y := (opcode & 0x00F0) >> 4
			V[x] = V[x] & V[y]
			goToNextInstruction()
		case 0x0003:	//0x8XY3: Sets VX to VX xor VY
			x := (opcode & 0x0F00) >> 8
			y := (opcode & 0x00F0) >> 4
			V[x] = V[x] ^ V[y]
			goToNextInstruction()
		case 0x0004:	//0x8XY4: Adds VY to VX. VF is set to 1 when there's a carry and to 0 otherwise.
			x := (opcode & 0x0F00) >> 8
			y := (opcode & 0x00F0) >> 4
			if V[y] > 0xFF - V[x]{	//V[y] greater than what is left to V[x] to overflow 0xFF (255)
				V[0xF] = 1
			}else{
				V[0xF] = 0
			}
			V[x] += V[y]
			goToNextInstruction()
		case 0x0005:	//0x8XY5: Subtracts VY from VX. VF is set to 0 when there's a borrow and to 1 otherwise.
			x := (opcode & 0x0F00) >> 8
			y := (opcode & 0x00F0) >> 4
			if V[y] > V[x]{
				V[0xF] = 0
			}else{
				V[0xF] = 1
			}
			V[x] -= V[y]
			goToNextInstruction()
		case 0x0006:	//0x8XY6: Shifts VX right by one. VF is set to the value of the least
				//significant bit of VX before the shift.
			x := (opcode & 0x0F00) >> 8
			V[0xF] = V[x] & 0x1
			V[x] = V[x] >> 1
			goToNextInstruction()
		case 0x0007:	//0x8XY7: Sets VX to VY minus VX. VF is set to 0 when there's a borrow and to 1 otherwise
			x := (opcode & 0x0F00) >> 8
			y := (opcode & 0x00F0) >> 4
			if V[x] > V[y]{
				V[0xF] = 0
			}else{
				V[0xF] = 1
			}
			V[x] = V[y] - V[x]
			goToNextInstruction()
		case 0x000E:	//0x8XYE: Shifts VX left by one. VF is set to the value of the most significant bit of VX
				//before the shift.
			x := (opcode & 0x0F00) >> 8
			V[0xF] = V[x] >> 7
			V[x] = V[x] << 1
			goToNextInstruction()
		}
	case 0x9000:	//0x9XY0: Skips the next instruction if VX doesn't equal VY
		x := (opcode & 0x0F00) >> 8
		y := (opcode & 0x00F0) >> 4
		if V[x] != V[y]{
			skipNextInstruction()
		}else{
			goToNextInstruction()
		}
	case 0xA000:	//0xANNN: Sets I to the address NNN
		I = opcode & 0x0FFF
		goToNextInstruction()
	case 0xB000:	//0xBNNN: Jumps to the address NNN plus V0
		jump((opcode & 0x0FFF) + uint16(V[0]))
	case 0xC000:	//0xCXNN: Sets VX to a random number, masked by NN
		x := (opcode & 0x0F00) >> 8
		nn := opcode & 0x00FF
		V[x] = uint8(uint16(rand.Float64() * float64(255)) & nn)
		goToNextInstruction()
	case 0xD000:	//0xDXYN: Draw sprite at coordinates VX VY. Sprites stored in memory at location in index register (I),
			//maximum 8bit wide. Wraps around the screen. If when drawn, clears a
			//pixel, register VF is set to 1 otherwise it is zero. All drawing is
			//XOR drawing (toggle mode)
		drawSprite()
		goToNextInstruction()
	case 0xE000:
		switch opcode & 0x00FF{
			case 0x009E:	//0xEX9E: Skip next instruction if the key stored in VX is pressed
				x := (opcode & 0x0F00) >> 8
				if key[V[x]] != 0{
					skipNextInstruction()
				}else{
					goToNextInstruction()
				}
			case 0x00A1:	//0xEXA1: Skips next instruction if the key stored in VX is not pressed
				x := (opcode & 0x0F00) >> 8
				if key[V[x]] == 0{
					skipNextInstruction()
				}else{
					goToNextInstruction()
				}
		}
	case 0xF000:
		switch opcode & 0x00FF{
			case 0x0007:	//0xFX07: Sets VX to the value of the delay timer
				x := (opcode & 0x0F00) >> 8
				V[x] = delayTimer
				goToNextInstruction()
			case 0x000A:	//0xFX0A: A key press is awaited, and then stored in VX
				x := (opcode & 0x0F00) >> 8
				keyPressed := false
				for i := 0; i < 16; i++{
					if key[i] != 0{
						V[x] = uint8(i)
						keyPressed = true
					}
				}
				if !keyPressed{
					return
				}
				goToNextInstruction()
			case 0x0015:	//0xFX15: Sets the delay timer to VX
				x := (opcode & 0x0F00) >> 8
				delayTimer = V[x]
				goToNextInstruction()
			case 0x0018:	//0xFX18: Sets the sound timer to VX
				x := (opcode & 0x0F00) >> 8
				soundTimer = V[x]
				goToNextInstruction()
			case 0x001E:	//0xFX1E: Adds VX to I
				x := (opcode & 0x0F00) >> 8
				I += uint16(V[x])
				goToNextInstruction()
			case 0x0029:	//0xFX29: Sets I to the location of the sprite for the
					//character in VX. Characters 0-F(hex) are represented by a 4x5 font.
				x := (opcode & 0x0F00) >> 8
				I = uint16(V[x] * 0x5)
				goToNextInstruction()
			case 0x0033:	//0xFX33: Stores the binary-coded decimal representation of VX at the addresses I, I+1, I+2
				x := (opcode & 0x0F00) >> 8
				memory[I]	= V[x] / 100
				memory[I + 1]	= (V[x] / 10) % 10
				memory[I + 2]	= (V[x] % 100) % 10
				goToNextInstruction()
			case 0x0055:	//0xFX55: Stores V0 to VX in memory starting at address I
				x := (opcode & 0x0F00) >> 8
				for i := 0; i <= int(x); i++{
					memory[I + uint16(i)] = V[i]
				}

				I += x + 1
				goToNextInstruction()
			case 0x0065:	//0xFX65: Fills V0 to VX with values from memory starting at address I
				x := (opcode & 0x0F00) >> 8
				for i := 0; i <= int(x); i++{
					V[i] = memory[I + uint16(i)]
				}

				I += x + 1
				goToNextInstruction()
			default:
				fmt.Printf("Unknown or unsupported opcode [0xF000]: 0x%d\n", opcode)
		}
	default:
		fmt.Printf("Unknown or unsupported opcode: 0x%d\n", opcode)
	}

	updateTimers()
}

func goToNextInstruction(){
	pc += 2			//jumps to the next instruction.
				//+2 is because instructions are 2 byte sized
}

func clearScreen(){
	for i, _ := range gfx{	//iterate through each pixel
		gfx[i] = 0;	//reset it's value to 0
	}
}

func jump(address uint16){
	pc = address		//sets program counter to the address requested
				//there's no stack involved from what I understood
				//so, it's possibly implemented in the wrong way.
				//needs testing!
}

func call(address uint16){
	stack[sp] = pc		//stores program counter into the top of the stack
	sp++			//moves stack pointer to the next empty address
	pc = address		//finally, jump to the requested address.
}

func skipNextInstruction(){
	goToNextInstruction()	//jumps to next instruction (after this one)
	goToNextInstruction()	//jumps the next instruction aswell
}

func drawSprite(){
	x := V[(opcode & 0x0F00) >> 8]
	y := V[(opcode & 0x00F0) >> 4]
	height := opcode & 0x000F
	var pixel uint16

	V[0xF] = 0
	for yline := 0; yline < int(height); yline++{
		pixel = uint16(memory[I + uint16(yline)])
		for xline := 0; xline < int(height); xline++{
			if pixel & (0x80 >> uint(xline)) != 0{
				if gfx[(x + uint8(xline) + ((y + uint8(yline)) * 64))] == 1{
					V[0xF] = 1
				}
				gfx[x + uint8(xline) + ((y + uint8(yline)) * 64)] ^= 1
			}
		}
	}

	drawFlag = true
}

func updateTimers(){
	if delayTimer > 0{
		delayTimer--
	}

	if soundTimer > 0{
		if soundTimer == 1{
			fmt.Printf("BEEP!\n")
		}
		soundTimer--
	}
}

func loadApplication(filename string){
	fmt.Printf("Loading file into memory: %s", filename)

	file, _ := os.Open(filename)

	reader := bufio.NewReader(file)
	i := 0
	for{
		c, e := reader.ReadByte()
		if e != nil{
			break
		}
		memory[i + 512] = c
		i++
		if i + 512 >= 4096{
			fmt.Printf("Out of memory! ROM is bigger than memory.")
			break
		}
	}
}

func initilize(){
	pc	= 0x200
	opcode	= 0
	I	= 0
	sp	= 0

	for i := 0; i < 4096; i++{
		memory[i] = 0
	}

	for i := 0; i < 16; i++{
		stack[i] = 0
	}

	clearScreen()

	chip8Fontset := [...]uint8{
    0xF0, 0x90, 0x90, 0x90, 0xF0, //0
    0x20, 0x60, 0x20, 0x20, 0x70, //1
    0xF0, 0x10, 0xF0, 0x80, 0xF0, //2
    0xF0, 0x10, 0xF0, 0x10, 0xF0, //3
    0x90, 0x90, 0xF0, 0x10, 0x10, //4
    0xF0, 0x80, 0xF0, 0x10, 0xF0, //5
    0xF0, 0x80, 0xF0, 0x90, 0xF0, //6
    0xF0, 0x10, 0x20, 0x40, 0x40, //7
    0xF0, 0x90, 0xF0, 0x90, 0xF0, //8
    0xF0, 0x90, 0xF0, 0x10, 0xF0, //9
    0xF0, 0x90, 0xF0, 0x90, 0x90, //A
    0xE0, 0x90, 0xE0, 0x90, 0xE0, //B
    0xF0, 0x80, 0x80, 0x80, 0xF0, //C
    0xE0, 0x90, 0x90, 0x90, 0xE0, //D
    0xF0, 0x80, 0xF0, 0x80, 0xF0, //E
    0xF0, 0x80, 0xF0, 0x80, 0x80} //F

	for i := 0; i < 80; i++{
		memory[i] = chip8Fontset[i];
	}

	delayTimer = 0
	soundTimer = 0

	drawFlag = true
}

func render(){
	for y := 0; y < 32; y++{
		for x := 0; x < 64; x++{
			if gfx[(y * 64) + x] == 1{
				fmt.Printf("█")
			}else{
				fmt.Printf(" ")
			}
		}
		fmt.Printf("\n")
	}
	fmt.Printf("\n")

	drawFlag = false
}

func main() {
	initilize()
	loadApplication("games/PONG2")


	for{
		if drawFlag{
			fmt.Print("\033[2J")	//ANSI clear screen
			render()
		}
		cycle()
		time.Sleep(1000 / 20 * time.Millisecond)
	}
}
