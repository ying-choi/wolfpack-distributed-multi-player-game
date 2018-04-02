package impl

import (
	"../../shared"
	"../../geometry"
	"fmt"
	"crypto/ecdsa"
)

// The "main" node part of the logic node. Deals with computation and checks; not communications
type PlayerNode struct {
	pixelInterface	  PixelInterface
	nodeInterface 	  *NodeCommInterface
	playerCommChannel chan string
	playerSendChannel chan shared.GameState
	GameState		  shared.GameState
	geo        geometry.GridManager
	Identifier string
	GameConfig shared.InitialState
}

// Creates the main logic node and required interfaces with the arguments passed in logic-node.go
// nodeListenerAddr = where we expect to receive messages from other nodes
// playerListenerAddr = where we expect to receive messages from the pixel-node
// pixelSendAddr = where we will be sending new game states to the pixel node
func CreatePlayerNode(nodeListenerAddr, playerListenerAddr string,
	pubKey *ecdsa.PublicKey, privKey *ecdsa.PrivateKey, serverAddr string) (PlayerNode) {
	// Setup the player communication buffered channel
	playerCommChannel := make(chan string, 5)
	playerSendChannel := make(chan shared.GameState, 5)

	// Start the node to node interface
	nodeInterface := CreateNodeCommInterface(pubKey, privKey, serverAddr)
	addr, listener := StartListenerUDP(nodeListenerAddr)

	nodeInterface.LocalAddr = addr
	nodeInterface.IncomingMessages = listener
	go nodeInterface.RunListener(listener, nodeListenerAddr)
	go nodeInterface.ManageOtherNodes()
	go nodeInterface.ManageAcks()
	go nodeInterface.PruneNodes()
	go nodeInterface.SendGameStateToPixel()

	// Register with server, update info
	uniqueId := nodeInterface.ServerRegister()
	go nodeInterface.SendHeartbeat()

	// Startup Pixel interface + listening
	pixelInterface := CreatePixelInterface(playerCommChannel, playerSendChannel,
		nodeInterface.Config.InitState.Settings, uniqueId)

	//// Make a gameState
	playerLocs := make(map[string]shared.Coord)
	playerLocs["prey"] = shared.Coord{5,5}
	playerLocs[uniqueId] = shared.Coord{1,1}

	playerMap := shared.PlayerLockMap{Data:playerLocs}

	// Make a gameState
	gameState := shared.GameState{
		PlayerLocs: playerMap,
	}

	// Create player node
	pn := PlayerNode{
		pixelInterface:    pixelInterface,
		nodeInterface:     &nodeInterface,
		playerCommChannel: playerCommChannel,
		playerSendChannel:playerSendChannel,
		geo:               geometry.CreateNewGridManager(nodeInterface.Config.InitState.Settings),
		GameState:         gameState,
		Identifier:        uniqueId,
		GameConfig:        nodeInterface.Config.InitState,
	}

	// Allow the node-node interface to refer back to this node
	nodeInterface.PlayerNode = &pn

	return pn
}

// Runs the main node (listens for incoming messages from pixel interface) in a loop, must be called at the
// end of main (or alternatively, in a goroutine)
func (pn * PlayerNode) RunGame(playerListener string) {
	go pn.pixelInterface.RunPlayerListener(playerListener)
	fmt.Println("listener running")

	for {
		message := <-pn.playerCommChannel
		switch message {
		case "quit":
			break
		default:
			move, didMove := pn.movePlayer(message)
			if didMove {
				pn.nodeInterface.SendMoveToNodes(&move)
			}
			// pn.pixelInterface.SendPlayerGameState(pn.GameState)
		}
	}

}

// Given a string "up"/"down"/"left"/"right", changes the player state to make that move iff that move is valid
// (not into a wall, out of bounds)
func (pn * PlayerNode) movePlayer(move string) (newPos shared.Coord, changed bool) {
	// Get current player state
	pn.GameState.PlayerLocs.RLock()
	playerLoc := pn.GameState.PlayerLocs.Data[pn.Identifier]
	pn.GameState.PlayerLocs.RUnlock()

	originalPosition := shared.Coord{X: playerLoc.X, Y: playerLoc.Y}
	// Calculate new position with move
	newPosition := shared.Coord{X: playerLoc.X, Y: playerLoc.Y}
	switch move {
	case "up":
		newPosition.Y = newPosition.Y + 1
	case "down":
		newPosition.Y = newPosition.Y - 1
	case "left":
		newPosition.X = newPosition.X - 1
	case "right":
		newPosition.X = newPosition.X + 1
	}
	// Check new move is valid, if so update player position
	if pn.geo.IsValidMove(newPosition) && pn.geo.IsNotTeleporting(originalPosition, newPosition){
		//pn.GameState.PlayerLocs.Lock()
		//pn.GameState.PlayerLocs.Data[pn.Identifier] = newPosition
		//pn.GameState.PlayerLocs.Unlock()
		return newPosition, true
	}
	return playerLoc, false
}

// GETTERS

func (pn *PlayerNode) GetPixelInterface() (PixelInterface) {
	return pn.pixelInterface
}

func (pn *PlayerNode) GetNodeInterface() (*NodeCommInterface) {
	return pn.nodeInterface
}
