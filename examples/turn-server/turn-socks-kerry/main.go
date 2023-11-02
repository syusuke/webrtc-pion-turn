// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

// Package main implements a simple TURN server
package main

import (
	"flag"
	"github.com/pion/turn/v3"
	"log"
	"net"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"strconv"
	"syscall"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	publicIP := flag.String("public-ip", "", "IP Address that TURN can be contacted by.")
	port := flag.Int("port", 3478, "Listening port.")
	users := flag.String("users", "", "List of username and password (e.g. \"user=pass,user=pass\")")
	realm := flag.String("realm", "pion.ly", "Realm (defaults to \"pion.ly\")")
	multiThread := flag.Bool("multi-thread", false, "Whether to use multithreading")
	threadCount := flag.Int("thread-count", 0, "when multithreading is true, custom set thread count,default CPU count")
	flag.Parse()

	if len(*publicIP) == 0 {
		log.Fatalf("'public-ip' is required")
	} else if len(*users) == 0 {
		log.Fatalf("'users' is required")
	}

	// Cache -users flag for easy lookup later
	// If passwords are stored they should be saved to your DB hashed using turn.GenerateAuthKey
	usersMap := map[string][]byte{}
	for _, kv := range regexp.MustCompile(`(\w+)=(\w+)`).FindAllStringSubmatch(*users, -1) {
		usersMap[kv[1]] = turn.GenerateAuthKey(kv[1], *realm, kv[2])
	}

	if *multiThread {
		if *threadCount == 0 {
			*threadCount = runtime.NumCPU()
		}
		log.Printf("threadCount is %v\n", *threadCount)

		if *threadCount > 1 {
			multiThreadSocket(publicIP, port, usersMap, realm, threadCount)
		} else {
			singleThreadSocket(publicIP, port, usersMap, realm)
		}
	} else {
		singleThreadSocket(publicIP, port, usersMap, realm)
	}
}

func singleThreadSocket(publicIP *string, port *int, usersMap map[string][]byte, realm *string) {

	log.Printf("start TURN server in single thread mode. addr = %v:%v\n", *publicIP, *port)

	udpListener, err := net.ListenPacket("udp4", "0.0.0.0:"+strconv.Itoa(*port))
	if err != nil {
		log.Panicf("Failed to create UDP TURN server listener: %s", err)
	}
	log.Printf("create UDP TURN server listener at port %d\n", *port)

	tcpListener, err := net.Listen("tcp4", "0.0.0.0:"+strconv.Itoa(*port))
	if err != nil {
		log.Panicf("Failed to create TURN server listener: %s", err)
	}
	log.Printf("create TCP TURN server listener at port %d\n", *port)

	relayAddressGenerator := &turn.RelayAddressGeneratorStatic{
		RelayAddress: net.ParseIP(*publicIP), // Claim that we are listening on IP passed by user
		Address:      "0.0.0.0",              // But actually be listening on every interface
	}

	s, err := turn.NewServer(turn.ServerConfig{
		Realm: *realm,
		AuthHandler: func(username string, realm string, srcAddr net.Addr) ([]byte, bool) {
			if key, ok := usersMap[username]; ok {
				return key, true
			}
			return nil, false
		},
		// PacketConnConfigs is a list of UDP Listeners and the configuration around them
		PacketConnConfigs: []turn.PacketConnConfig{
			{
				PacketConn:            udpListener,
				RelayAddressGenerator: relayAddressGenerator,
			},
		},
		// ListenerConfig is a list of Listeners and the configuration around them
		ListenerConfigs: []turn.ListenerConfig{
			{
				Listener:              tcpListener,
				RelayAddressGenerator: relayAddressGenerator,
			},
		},
	})

	if err != nil {
		log.Panic(err)
	}

	log.Printf("create TURN server success\n")

	// // Block until user sends SIGINT or SIGTERM
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs

	if err = s.Close(); err != nil {
		log.Panic(err)
	}
}

func multiThreadSocket(publicIP *string, port *int, usersMap map[string][]byte, realm *string, threadNum *int) {

	log.Printf("start TURN server in multi thread mode. threadNum = %v. addr = %v:%v\n", *threadNum, *publicIP, *port)

	log.Panicf("no impp for multi thread mode")
}
