package commands

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/maticnetwork/libp2p-gossip-bench/agent"
	lat "github.com/maticnetwork/libp2p-gossip-bench/latency"
	"github.com/maticnetwork/libp2p-gossip-bench/network"
	"github.com/mitchellh/cli"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	linear       = "linear"
	random       = "random"
	superCluster = "super-cluster"
)

const (
	IpString                = "127.0.0.1"
	outputFileDirectory     = "/tmp"
	RandomConnectionsCount  = 2000
	RandomTopologyConnected = true
	MaxPeers                = 30
)

//StartGossipCommand is a struct containing data for running framework
type StartGossipCommand struct {
	UI cli.Ui

	nodeCount      int
	validatorCount int
	topology       string
	messageRate    time.Duration
	benchDuration  time.Duration
	messageSize    int
	peeringDegree  int
	startingPort   int
}

// Help implements the cli.Command interface
func (fc *StartGossipCommand) Help() string {
	return `Command runs the libp2p framework based on provided configuration (node count, validator count, ).
	
	Usage: start -nodes={numberOfNodes} -validators={numberOfvalidators}  -topology={topologyType(linear,ranodm, super-cluster)} -rate={messagesRate} -size={messageSize}
	
	Options:
	
	-nodes 		- Count of nodes
	-validators - Count of validators
	-topology 	- Topology of the nodes (linear, random)
	-rate 		- Message rate of a node in miliseconds
	-duration   - Benchmanr duration in miliseconds
	-size 		- Size of a transmited message
	-degree 	- Peering degree:Count of directly connected peers`
}

// Synopsis implements the cli.Command interface
func (fc *StartGossipCommand) Synopsis() string {
	return "Starts the libp2p framework"
}

// Run implements the cli.Command interface and runs the command
func (fc *StartGossipCommand) Run(args []string) int {
	flagSet := fc.NewFlagSet()
	err := flagSet.Parse(args)
	if err != nil {
		fc.UI.Error(err.Error())
		return 1
	}

	fc.UI.Info("Starting libp2p benchmark ...")
	fc.UI.Info(fmt.Sprintf("Node count: %v\n", fc.nodeCount))
	fc.UI.Info(fmt.Sprintf("Validator count: %v\n", fc.validatorCount))
	fc.UI.Info(fmt.Sprintf("Chosen topology: %s\n", fc.topology))
	fc.UI.Info(fmt.Sprintf("Message rate (miliseconds): %v\n", fc.messageRate))
	fc.UI.Info(fmt.Sprintf("Benchmark duration (miliseconds): %v\n", fc.benchDuration))
	fc.UI.Info(fmt.Sprintf("Message size (bytes): %v\n", fc.messageSize))
	fc.UI.Info(fmt.Sprintf("Peering degree: %v\n", fc.peeringDegree))

	fc.UI.Info("Starting benchmark...")

	var topology agent.Topology
	switch fc.topology {
	case linear:
		topology = agent.LinearTopology{}
	case random:
		topology = agent.RandomTopology{
			Connected: RandomTopologyConnected,
			MaxPeers:  MaxPeers,
			Count:     RandomConnectionsCount,
		}
	case superCluster:
		topology = agent.SuperClusterTopology{
			ValidatorPeering:    uint(fc.peeringDegree),
			NonValidatorPeering: 1, // magic numner for now
		}
	default:
		fc.UI.Info(fmt.Sprintf("Unknown tpology %s submitted\n", fc.topology))
		return 1
	}

	StartGossipBench(fc.nodeCount, fc.validatorCount, fc.startingPort, fc.messageSize, fc.messageRate, fc.benchDuration, topology)

	fc.UI.Info("Benchmark executed")

	return 0
}

// NewFlagSet implements the interface and creates a new flag set for command arguments
func (fc *StartGossipCommand) NewFlagSet() *flag.FlagSet {
	flagSet := flag.NewFlagSet("libp2p-framework", flag.ContinueOnError)
	flagSet.IntVar(&fc.nodeCount, "nodes", 10, "Count of nodes")
	flagSet.IntVar(&fc.validatorCount, "validators", 2, "Count of validators")
	flagSet.StringVar(&fc.topology, "topology", "linear", fmt.Sprintf("Topology of the nodes (%s, %s, %s)", linear, random, superCluster))
	flagSet.DurationVar(&fc.messageRate, "rate", time.Millisecond*900, "Message rate (in miliseconds) of a node")
	flagSet.DurationVar(&fc.benchDuration, "duration", time.Millisecond*40000, "Duration of a benchmark in miliseconds")
	flagSet.IntVar(&fc.messageSize, "size", 4096, "Size (in bytes) of a transmited message")
	flagSet.IntVar(&fc.peeringDegree, "degree", 4, "Peering degree:Count of directly connected peers")
	flagSet.IntVar(&fc.startingPort, "port", 10000, "Port of the first agent")

	return flagSet
}

func StartGossipBench(agentsNumber, validatorsNumber, startingPort, msgSize int, msgRate, testTimeout time.Duration, toplogy agent.Topology) {
	// remove file if exists
	// logger configuration
	cfg := zap.NewProductionConfig()
	cfg.OutputPaths = []string{filepath.Join(outputFileDirectory, fmt.Sprintf("agents_%s.log", time.Now().Format(time.RFC3339)))}
	cfg.EncoderConfig = zapcore.EncoderConfig{
		TimeKey:    "time",
		MessageKey: "msg",
	}
	cfg.Sampling = nil
	cfg.EncoderConfig.EncodeTime = SyslogTimeEncoder

	logger, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	// flush buffer
	defer logger.Sync()

	connManager := network.NewConnManagerNetPipe()
	latencyData := lat.ReadLatencyDataFromJson()
	cluster := agent.NewCluster(logger, latencyData, agent.ClusterConfig{
		Ip:             IpString,
		StartingPort:   startingPort,
		MsgSize:        msgSize,
		ValidatorCount: validatorsNumber,
	})

	transportManager := network.NewTransportManager(connManager, cluster)

	fmt.Println("Output file path: ", cfg.OutputPaths)
	fmt.Println("Start adding agents: ", agentsNumber)

	// start agents in cluster
	acfg := agent.DefaultGossipConfig()
	acfg.Transport = transportManager.Transport()
	agentsAdded, agentsFailed, timeAdded := cluster.StartAgents(agentsNumber, *acfg)
	fmt.Printf("Agents added: %d. Failed agents: %v, Elapsed time: %v\n", agentsAdded, agentsFailed, timeAdded)
	cluster.ConnectAgents(toplogy)

	fmt.Println("Gossip started")
	msgsPublishedCnt, msgsFailedCnt := cluster.MessageLoop(context.Background(), msgRate, testTimeout)
	fmt.Printf("Published %d messages \n", msgsPublishedCnt)
	fmt.Printf("Failed %d messages \n", msgsFailedCnt)
}

func SyslogTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format(time.RFC3339))
}

func GetGossipCommands() map[string]cli.CommandFactory {
	ui := &cli.BasicUi{
		Reader:      os.Stdin,
		Writer:      os.Stdout,
		ErrorWriter: os.Stderr,
	}
	return map[string]cli.CommandFactory{
		"start": func() (cli.Command, error) {
			return &StartGossipCommand{
				UI: ui,
			}, nil
		},
	}
}