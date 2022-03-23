package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/maticnetwork/libp2p-gossip-bench/agent"
	lat "github.com/maticnetwork/libp2p-gossip-bench/latency"
	"github.com/maticnetwork/libp2p-gossip-bench/network"
)

const AgentsNumber = 5
const StartingPort = 10000
const MaxPeers = 2
const IpString = "127.0.0.1"

var logger *log.Logger = log.New(os.Stdout, "Mesh: ", log.Flags())

func main() {
	rand.Seed(time.Now().Unix())

	connManager := network.NewConnManagerNetPipe()
	latencyData := lat.ReadLatencyData()
	cluster := network.NewCluster(logger, latencyData, IpString, StartingPort, MaxPeers)
	transportManager := network.NewTransportManager(connManager, cluster)

	fmt.Println("Start adding agents")
	for i := 0; i < AgentsNumber; i++ {
		ac := &agent.AgentConfig{
			City:      latencyData.GetRandomCity(),
			Transport: transportManager.Transport(),
		}
		a := agent.NewAgent(logger, ac)
		_, err := cluster.AddAgent(a)
		if err != nil {
			panic(err)
		}
	}

	fmt.Println("Agents has been added")
	for i := 1; i <= AgentsNumber; i++ {
		size := AgentsNumber / MaxPeers
		for j := i + size; j <= AgentsNumber; j += size {
			fmt.Println("Connected ", i+StartingPort, " ", StartingPort+j)
			err := cluster.Connect(i+StartingPort, StartingPort+j)
			if err != nil {
				fmt.Println("Could not connect peers ", i+StartingPort, " ", StartingPort+j, " ", err)
			}
		}
	}

	fmt.Println("Gossip started")
	<-cluster.GossipLoop(context.Background(), time.Millisecond*900, time.Second*10)
}
