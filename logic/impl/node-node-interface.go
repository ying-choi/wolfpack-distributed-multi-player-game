package impl

import (
	"fmt"
	"net"
	"net/rpc"
	"log"
	"os"
	"../../shared"
	"encoding/json"
	"crypto/ecdsa"
	"time"
	"encoding/gob"
	"crypto/elliptic"
	"strconv"
)

// Node communication interface for communication with other player/logic nodes
type NodeCommInterface struct {
	PlayerNode			*PlayerNode
	PubKey 				*ecdsa.PublicKey
	PrivKey 			*ecdsa.PrivateKey
	Config 				shared.GameConfig
	ServerConn 			*rpc.Client
	IncomingMessages 	*net.UDPConn
	LocalAddr			net.Addr
	OtherNodes 			[]*net.Conn
	Connections 		[]string
}

type PlayerInfo struct {
	Address 			net.Addr
	PubKey 				ecdsa.PublicKey
}

// Creates a node comm interface with initial empty arrays
func CreateNodeCommInterface(pubKey *ecdsa.PublicKey, privKey *ecdsa.PrivateKey) (NodeCommInterface) {
	return NodeCommInterface {
		PubKey: pubKey,
		PrivKey: privKey,
		OtherNodes: make([]*net.Conn, 0),
		Connections: make([]string, 0)}
}

// Runs listener for messages from other nodes, should be run in a goroutine
func (n *NodeCommInterface) RunListener(nodeListenerAddr string) {
	// Start the listener
	addr, listener := StartListenerUDP(nodeListenerAddr)
	n.LocalAddr = addr
	n.IncomingMessages = listener
	listener.SetReadBuffer(1048576)

	i := 0
	for {
		i++
		buf := make([]byte, 1024)
		rlen, addr, err := listener.ReadFromUDP(buf)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(string(buf[0:rlen]))
		fmt.Println(addr)
		fmt.Println(i)
		// Write to the node comm channel
		if "sgs" == string(buf[0:3]){
			id := string(buf[3])
			if err != nil{
				panic(err)
			}
			var remoteCoord shared.Coord
			err2 := json.Unmarshal(buf[4:rlen], &remoteCoord)
			if err2 != nil {
				panic(err)
			}
			n.PlayerNode.GameRenderState.OtherPlayers[id] = remoteCoord
			fmt.Println(n.PlayerNode.GameRenderState.OtherPlayers)
		} else if string(buf[0:rlen]) != "connected" {
			remoteClient, err := net.Dial("udp", string(buf[0:rlen]))
			if err != nil {
				panic(err)
			}
			toSend, err := json.Marshal(n.PlayerNode.GameRenderState.PlayerLoc)
			// Code sgs sends the connecting node the gamestate
			remoteClient.Write([]byte("sgs" + n.PlayerNode.identifier + string(toSend)))
			n.OtherNodes = append(n.OtherNodes, &remoteClient)
		}
	}
}

// Registers the node with the server, receiving the gameconfig (and connections)
// TODO: maybe move this into node.go?
func (n *NodeCommInterface) ServerRegister() (id string) {
	gob.Register(&net.UDPAddr{})
	gob.Register(&elliptic.CurveParams{})
	gob.Register(&PlayerInfo{})

	if n.ServerConn == nil {
		// fmt.Printf("DEBUG - ServerRegister() n.ServerConn [%s] should be nil\n", n.ServerConn)
		// Connect to server with RPC, port is always :8081
		serverConn, err := rpc.Dial("tcp", ":8081")
		if err != nil {
			log.Println("Cannot dial server. Please ensure the server is running and try again.")
			os.Exit(1)
		}
		// Storing in object so that we can do other RPC calls outside of this function
		n.ServerConn = serverConn

		var response shared.GameConfig
		// Register with server
		playerInfo := PlayerInfo{n.LocalAddr, *n.PubKey}
		// fmt.Printf("DEBUG - PlayerInfo Struct [%v]\n", playerInfo)
		err = serverConn.Call("GServer.Register", playerInfo, &response)
		if err != nil {
			log.Fatal(err)
		}
		n.Config = response
	}

	n.GetNodes()

	// Start communcation with the other nodes
	go n.FloodNodes()

	return strconv.Itoa(n.Config.Identifier)
}

func (n *NodeCommInterface) GetNodes() {
	var response []net.Addr
	err := n.ServerConn.Call("GServer.GetNodes", *n.PubKey, &response)
	if err != nil {
		log.Fatal(err)
	}
	n.Connections = nil
	for _, addr := range response {
		n.Connections = append(n.Connections, addr.String())
	}
}

func (n *NodeCommInterface) SendHeartbeat() {
	for {
		var _ignored bool
		err := n.ServerConn.Call("GServer.Heartbeat", *n.PubKey, &_ignored)
		if err != nil {
			fmt.Printf("DEBUG - Heartbeat err: [%s]\n", err)
			n.ServerRegister()
		}
		boop := n.Config.GlobalServerHB
		time.Sleep(time.Duration(boop)*time.Microsecond)
	}
}

// Initiates connection with n.connections (provided nodes from server) on game init
func (n *  NodeCommInterface) FloodNodes() {
	const udpGeneric = "127.0.0.1:0"
	localIP, _ := net.ResolveUDPAddr("udp", udpGeneric)
	for _, ip := range n.Connections {
		nodeUdp, _ := net.ResolveUDPAddr("udp", ip)
		// Connect to other node
		nodeClient, err := net.DialUDP("udp", localIP, nodeUdp)
		if err != nil {
			panic(err)
		}
		// Exchange messages with other node
		myListener := n.LocalAddr.String()
		nodeClient.Write([]byte(myListener))
	}
}