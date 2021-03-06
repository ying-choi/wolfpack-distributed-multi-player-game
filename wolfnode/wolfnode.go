package wolfnode

import (
	"net/rpc"
	"crypto/ecdsa"
	"net"
	"../shared"
	"../key-helpers"
	"../wolferrors"
)

// A player information object
type PlayerInfo struct {
	ServerAddr       string
	ServerConn       *rpc.Client
	PubKey           *ecdsa.PublicKey
	PrivKey          *ecdsa.PrivateKey
	PlayerIP         net.Addr
	InitGameSettings shared.InitialGameSettings
	// game states are really just position coordinates + other info related to time synchro
	// we would also store this node's game state in here
	CurrGameState    map[string]shared.PlayerState
	// do we need this if we're using keys?
	PlayerId			uint32
	OtherPlayersConn	map[string]*rpc.Client
	// since we're using UDP to send msgs to/from player nodes, this is to track how many times we are unable to
	// reach another node. If it crosses the threshold, some number set by server?, then we delete from OtherPlayersConn
	OtherPlayersTracker	map[string]int
	MoveCommits			map[string]string
}

// Networking details + local checks
type WolfNode interface {
	// Register with server with a one-way node to server RPC connection.
	// Gets InitGameSettings, and PlayerId (this to be tbd)
	// Sends Pubkey and PlayerIP
	// Can return the following errors:
	// - DisconnectedError
	// - KeyAlreadyRegisteredError
	// - AddressAlreadyRegisteredError
	RegisterServer(serverAddr string) (err error)

	// Sets up a hearbeat protocol with the global server to let it know that this player is alive.
	// Can return the following errors:
	// - DisconnectedError
	SendHearbeat()

	// Returns the other players' connection information from global server.
	// Updates this node's OtherPlayersConn attribute (add to, or delete from).
	// Can return the following errors:
	// - DisconnectedError
	// - UnknownKeyError
	GetNodes() (err error)

	// Listens to UDP packets coming in from other players
	// https://stackoverflow.com/questions/28400340/how-support-concurrent-connections-with-a-udp-server-using-go
	Listen(conn *net.UDPConn, quit chan struct{})

	// Handle request from UDP connection, can be a move commit, a move, or an updated score
	HandleRequest(dontknowwhatformatyet string) (err error)

	// Connect to other player nodes given by GetNodes() method using UDP conn.
	// Will be stored in the player object's OtherPlayersConn attribute
	// Can return the following errors:
	// - DisconnectedError
	ConnectToOtherPlayerNodes() (err error)

	// Updates this node's OtherPlayersConn attribute (delete from) iff we do not receive a "I'm alive" message Ping times.
	// Can return the following errors:
	// - DisconnectedError
	TrackOtherPlayersNodes() (err error)

	///////////////////////////////////////////////////// CHECKS ///////////////////////////////////////////////////////

	// Check move to see if it's valid based on this node's game state.
	// Can return the following errors:
	// - InvalidMoveError
	// - OutOfBoundsError
	// - IncorrectPlayerError
	CheckMoveCommit(commitHash string, moveOp shared.MoveOp) (err error)

	// Check move to see if it's valid based on this node's game state.
	// Can return the following errors:
	// - InvalidMoveError
	// - OutOfBoundsError
	CheckMove(move shared.Coord) (err error)

	// Check move to see if they actually got the prey based on this node's game state.
	// Can return the following errors:
	// - InvalidPreyCaptureError
	CheckCapturedPrey() (err error)

	// Check update of high score is valid based on this node's game state.
	// Can return the following errors:
	// - InvalidScoreUpdateError
	CheckScore(score int) (err error)
}

// Methods that will utilize UDP to send info to other player nodes
type PlayerService interface {
	// Send a move commit to other players.
	// Can return the following errors:
	// - DisconnectedError
	SendMoveCommitment(commit shared.MoveOp) (err error)

	// Send moves to other players.
	// Can return the following errors:
	// - DisconnectedError
	SendMove(move shared.GameState) (err error)

	// Send updated score to other players after capturing prey.
	// Can return the following errors:
	// - DisconnectedError
	SendUpdatedScore(updatedScore uint32) (err error)
}

type WolfNodeImpl struct {
	Info	PlayerInfo
}

// Check move to see if they actually got the prey based on this node's game state.
func (wolfNode WolfNodeImpl) CheckCapturedPrey() (err error) {
	//preyX := prey.PreyRunner{}.GetPosition().X
	//preyY := prey.PreyRunner{}.GetPosition().Y
	preyX := 5 // TODO: change these once we implement prey
	preyY := 5 // TODO: change these once we implement prey
	_, publicKeyString := key_helpers.Encode(wolfNode.Info.PrivKey, wolfNode.Info.PubKey)
	coordinates := wolfNode.Info.CurrGameState[publicKeyString].PlayerLoc
	currX := coordinates.X
	currY := coordinates.Y
	if int(preyX) == currX && int(preyY) == currY {
		return nil
	}
	return wolferrors.InvalidPreyCaptureError("Gurl you did NOT get this prey")
}

// Check update of high score is valid based on this node's game state.
func (wolfNode WolfNodeImpl) CheckScore(score int) (err error) {
	// Check they actually scored
	captured := wolfNode.CheckCapturedPrey()
	if captured != nil {
		return wolferrors.InvalidScoreUpdateError(score)
	}

	// Must report accurate score
	if score != 1 { //TODO: change this if we want the score to not always be 1?
		return wolferrors.InvalidScoreUpdateError(score)
	}

	return nil
}