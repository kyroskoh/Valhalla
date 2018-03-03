package player

import (
	"log"

	"github.com/Hucaru/Valhalla/character"
	"github.com/Hucaru/Valhalla/connection"
	"github.com/Hucaru/Valhalla/interfaces"
	"github.com/Hucaru/gopacket"
)

func HandleConnect(conn interfaces.ClientConn, reader gopacket.Reader) uint32 {
	charID := reader.ReadUint32()

	char := character.GetCharacter(charID)
	char.SetEquips(character.GetCharacterEquips(char.GetCharID()))
	char.SetSkills(character.GetCharacterSkills(char.GetCharID()))
	char.SetItems(character.GetCharacterItems(char.GetCharID()))

	var isAdmin bool

	err := connection.Db.QueryRow("SELECT isAdmin from users where userID=?", char.GetUserID()).Scan(&isAdmin)

	if err != nil {
		panic(err)
	}

	channelID := uint32(0) // Either get from world server or have it be part of config file

	conn.SetAdmin(isAdmin)
	conn.SetIsLogedIn(true)
	conn.SetChanID(channelID)

	charsPtr.AddOnlineCharacter(conn, &char)

	conn.Write(enterGame(char, channelID))

	log.Println(char.GetName(), "has loged in from", conn)

	return char.GetCurrentMap()
}

func HandleMovement(conn interfaces.ClientConn, reader gopacket.Reader) (uint32, gopacket.Packet) {
	// http://mapleref.wikia.com/wiki/Movement
	/*
		State enum:
			left / right: Action
			3 / 2: Walk
			5 / 4: Standing
			7 / 6: Jumping & Falling
			9 / 8: Normal attack
			11 / 10: Prone
			13 / 12: Rope
			15 / 14: Ladder
	*/
	reader.ReadBytes(5) // used in movement validation
	char := charsPtr.GetOnlineCharacterHandle(conn)

	nFragaments := reader.ReadByte()

	for i := byte(0); i < nFragaments; i++ {
		movementType := reader.ReadByte()

		switch movementType { // Movement type
		// Absolute movement
		case 0x00: // normal move
			fallthrough
		case 0x05: // normal move
			fallthrough
		case 0x17:
			posX := reader.ReadInt16()
			posY := reader.ReadInt16()

			velX := reader.ReadInt16()
			velY := reader.ReadInt16()

			foothold := reader.ReadUint16()

			state := reader.ReadByte()
			duration := reader.ReadUint16()

			char.SetX(posX + velX*int16(duration))
			char.SetY(posY + velY*int16(duration))
			char.SetFh(foothold)
			char.SetState(state)

		// Relative movement
		case 0x01: // jump
			fallthrough
		case 0x02:
			fallthrough
		case 0x06:
			fallthrough
		case 0x12:
			fallthrough
		case 0x13:
			fallthrough
		case 0x16:
			reader.ReadInt16() // velX
			reader.ReadInt16() // velY

			state := reader.ReadByte()
			foothold := reader.ReadUint16()

			char.SetState(state)
			char.SetFh(foothold)

		// Instant movement
		case 0x03:
			fallthrough
		case 0x04: // teleport
			fallthrough
		case 0x07: // assaulter
			fallthrough

		case 0x09:
			fallthrough
		case 0x014:
			posX := reader.ReadInt16()
			posY := reader.ReadInt16()
			reader.ReadInt16() // velX
			reader.ReadInt16() // velY

			state := reader.ReadByte()

			char.SetX(posX)
			char.SetY(posY)
			char.SetState(state)

		// Equip movement
		case 0x10:
			reader.ReadByte() // ?

		// Jump down movement
		case 0x11:
			posX := reader.ReadInt16()
			posY := reader.ReadInt16()
			velX := reader.ReadInt16()
			velY := reader.ReadInt16()

			reader.ReadUint16()

			foothold := reader.ReadUint16()
			duration := reader.ReadUint16()

			char.SetX(posX + velX*int16(duration))
			char.SetY(posY + velY*int16(duration))
			char.SetFh(foothold)
		case 0x08:
			reader.ReadByte()
		default:
			log.Println("Unkown movement type received", movementType, reader.GetRestAsBytes())
		}
	}

	reader.GetRestAsBytes() // used in movement validation

	return char.GetCurrentMap(), playerMovePacket(char.GetCharID(), reader.GetBuffer()[2:])
}
