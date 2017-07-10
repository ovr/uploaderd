package main

import (
	zmq "github.com/pebbe/zmq4"
	"strconv"
	"sync"
)

var (
	// todo, @ovr should rewrite this shit!
	clientMutex sync.Mutex
)

func tryUUID(client *zmq.Socket) (uint64, error) {
	clientMutex.Lock()
	defer clientMutex.Unlock()

	_, err := client.SendMessage("GEN")
	if err != nil {
		return 0, err
	}

	reply, err := client.RecvMessage(0)
	if err != nil {
		return 0, err
	}

	res, err := strconv.ParseUint(reply[0], 10, 64)
	if err != nil {
		return 0, err
	}

	return res, nil
}

func generateUUID(client *zmq.Socket) uint64 {
	for i := 0; i < 5; i++ {
		res, err := tryUUID(client)
		if err == nil {
			return res
		}
	}

	panic("Cannot generate UUID after N tries")
}
