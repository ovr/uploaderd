package main

import (
	"fmt"
	zmq "github.com/pebbe/zmq4"
	"log"
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

const TRIES_COUNT = 5

func generateUUID(client *zmq.Socket) uint64 {
	var err error

	for i := 0; i < TRIES_COUNT; i++ {
		res, err := tryUUID(client)
		if err == nil {
			return res
		} else {
			// Log error and give a new try
			log.Println(err.Error())
		}
	}

	panic(fmt.Sprintf("Cannot generate UUID after %d tries (%s)", TRIES_COUNT, err.Error()))
}
