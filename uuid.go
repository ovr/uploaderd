package main

import (
	"fmt"
	zmq "github.com/pebbe/zmq4"
	"log"
	"strconv"
)

const TRIES_COUNT = 5

type UUIDGenerator struct {
	client *zmq.Socket
	recv   chan uint64
}

func NewUUIDGenerator(endpoint string) *UUIDGenerator {
	zmqClient, err := zmq.NewSocket(zmq.DEALER)
	if err != nil {
		panic(err)
	}

	err = zmqClient.Connect(endpoint)
	if err != nil {
		panic(err)
	}

	client := &UUIDGenerator{
		client: zmqClient,
		recv:   make(chan uint64, 5),
	}

	return client
}

func (this *UUIDGenerator) Listen() {
	for {
		var err error

		for i := 0; i < TRIES_COUNT; i++ {
			res, err := this.tryUUID()
			if err == nil {
				this.recv <- res

				log.Println("Next UUID ", res)

				// next loop
				continue
			} else {
				// Log error and give a new try
				log.Println(err.Error())
			}
		}

		panic(fmt.Sprintf("Cannot generate UUID after %d tries (%s)", TRIES_COUNT, err.Error()))
	}
}

func (this *UUIDGenerator) tryUUID() (uint64, error) {
	_, err := this.client.SendMessage("GEN")
	if err != nil {
		return 0, err
	}

	reply, err := this.client.RecvMessage(0)
	if err != nil {
		return 0, err
	}

	res, err := strconv.ParseUint(reply[0], 10, 64)
	if err != nil {
		return 0, err
	}

	return res, nil
}

func (this *UUIDGenerator) Get() uint64 {
	return <-this.recv
}
