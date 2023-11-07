// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

// Package main implements a simple TURN server
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/pion/turn/v3"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

type Configuration struct {
	PublicIp    string            `json:"public-ip"`
	Port        int               `json:"port"`
	UsersMap    map[string]string `json:"users-map"`
	Realm       string            `json:"realm"`
	LogConsole  bool              `json:"log-console"`
	LogFile     bool              `json:"log-file"`
	LogFilePath string            `json:"log-file-path"`
	MultiThread bool              `json:"multi-thread"`
	ThreadCount int               `json:"thread-count"`
	SocketType  string            `json:"socket-type"` // (tcp),(udp),(tcp,udp)
}

var configuration = &Configuration{
	Port:        3478,
	Realm:       "kerry",
	LogConsole:  true,
	LogFile:     false,
	LogFilePath: "./webrtc-turn-server.log",
	MultiThread: false,
	ThreadCount: 0,
	SocketType:  "tcp,udp",
}

func loadConfig() {
	configPath := flag.String("config", "", "Json config file path.")
	publicIP := flag.String("public-ip", "", "IP Address that TURN can be contacted by.")
	port := flag.Int("port", -1, "Listening port.")
	realm := flag.String("realm", "kerry", "Realm (defaults to \"kerry\")")
	users := flag.String("users", "", "List of username and password (e.g. \"user=pass,user=pass\")")
	socketType := flag.String("socket-type", "", "support socket type (e.g. \"tcp; udp; tcp,udp\")")
	flag.Parse()

	if len(*configPath) > 0 {
		jsonConfigFile, err := os.Open(*configPath)
		defer func(configFile *os.File) {
			err := configFile.Close()
			if err != nil {
				panic(err)
			}
		}(jsonConfigFile)
		if err != nil {
			panic(err)
		}
		jsonBytes, err := io.ReadAll(jsonConfigFile)
		if err != nil {
			panic(err)
		}
		err = json.Unmarshal(jsonBytes, &configuration)
		if err != nil {
			panic(err)
		}
	}

	if configuration.UsersMap == nil {
		configuration.UsersMap = make(map[string]string)
	}
	// over write config
	if len(*publicIP) > 0 {
		configuration.PublicIp = *publicIP
	}
	if *port > 0 {
		configuration.Port = *port
	}
	if len(*realm) > 0 {
		configuration.Realm = *realm
	}
	if len(*users) > 0 {
		for _, kv := range regexp.MustCompile(`(\w+)=(\w+)`).FindAllStringSubmatch(*users, -1) {
			configuration.UsersMap[kv[1]] = kv[2]
		}
	}
	if len(*socketType) > 0 {
		configuration.SocketType = *socketType
	}
}

func checkConfig() {
	if len(configuration.PublicIp) == 0 {
		log.Fatalf("'public-ip' is required")
	}
	if len(configuration.UsersMap) == 0 {
		log.Fatalf("'users-map' is required")
	}
}

func main() {
	loadConfig()
	// config log
	if configuration.LogFile {
		fmt.Printf("logFile. %v\n", configuration.LogFilePath)
		f, err := os.OpenFile(configuration.LogFilePath, os.O_CREATE|os.O_APPEND|os.O_RDWR, os.ModePerm)
		if err != nil {
			fmt.Printf("logFile. %v\n", err)
			return
		}
		defer func(f *os.File) {
			err := f.Close()
			if err != nil {

			}
		}(f)

		if configuration.LogConsole {
			io.MultiWriter(os.Stdout, f)
		} else {
			log.SetOutput(f)
		}
		log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	} else {
		log.SetOutput(os.Stdout)
		log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	}

	// start and check config
	log.Printf("configuration: %+v\n", configuration)
	checkConfig()

	usersMap := map[string][]byte{}
	for username, password := range configuration.UsersMap {
		usersMap[username] = turn.GenerateAuthKey(username, configuration.Realm, password)
	}

	if configuration.MultiThread {
		var threadNum int
		if configuration.ThreadCount == 0 {
			threadNum = runtime.NumCPU()
		} else {
			threadNum = configuration.ThreadCount
		}
		log.Printf("multi thread mode threadNum is %v\n", threadNum)

		if threadNum > 1 {
			multiThreadSocket(usersMap, threadNum)
		} else {
			singleThreadSocket(usersMap)
		}
	} else {
		singleThreadSocket(usersMap)
	}
}

func singleThreadSocket(usersMap map[string][]byte) {

	log.Printf("start TURN server in single thread mode. addr = %v:%v\n", configuration.PublicIp, configuration.Port)

	relayAddressGenerator := &turn.RelayAddressGeneratorStatic{
		RelayAddress: net.ParseIP(configuration.PublicIp), // Claim that we are listening on IP passed by user
		Address:      "0.0.0.0",                           // But actually be listening on every interface
	}

	// udp
	var packetConnConfigs []turn.PacketConnConfig
	if strings.Contains(configuration.SocketType, "udp") {
		udpListener, err := net.ListenPacket("udp4", "0.0.0.0:"+strconv.Itoa(configuration.Port))
		if err != nil {
			log.Panicf("Failed to create UDP TURN server listener: %s", err)
		}
		log.Printf("create UDP TURN server listener at port %d\n", configuration.Port)
		packetConnConfigs = []turn.PacketConnConfig{
			{
				PacketConn:            udpListener,
				RelayAddressGenerator: relayAddressGenerator,
			},
		}
	} else {
		packetConnConfigs = make([]turn.PacketConnConfig, 0)
	}

	// tcp
	var listenerConfigs []turn.ListenerConfig
	if strings.Contains(configuration.SocketType, "udp") {
		tcpListener, err := net.Listen("tcp4", "0.0.0.0:"+strconv.Itoa(configuration.Port))
		if err != nil {
			log.Panicf("Failed to create TURN server listener: %s", err)
		}
		log.Printf("create TCP TURN server listener at port %d\n", configuration.Port)
		listenerConfigs = []turn.ListenerConfig{
			{
				Listener:              tcpListener,
				RelayAddressGenerator: relayAddressGenerator,
			},
		}
	} else {
		listenerConfigs = make([]turn.ListenerConfig, 0)
	}

	if len(packetConnConfigs) == 0 && len(listenerConfigs) == 0 {
		log.Panic("socketType must be tcp or udp or both(tcp,udp)")
	}

	s, err := turn.NewServer(turn.ServerConfig{
		Realm: configuration.Realm,
		AuthHandler: func(username string, realm string, srcAddr net.Addr) ([]byte, bool) {
			if key, ok := usersMap[username]; ok {
				return key, true
			}
			return nil, false
		},
		PacketConnConfigs: packetConnConfigs,
		ListenerConfigs:   listenerConfigs,
	})

	if err != nil {
		log.Panic(err)
	}

	log.Printf("create TURN server success\n")

	// Block until user sends SIGINT or SIGTERM
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs

	if err = s.Close(); err != nil {
		log.Panic(err)
	}
}

func multiThreadSocket(usersMap map[string][]byte, threadNum int) {

	log.Printf("start TURN server in multi thread mode. threadNum = %v. addr = %v:%v\n", threadNum, configuration.PublicIp, configuration.Port)

	log.Panicf("no impp for multi thread mode")
}
