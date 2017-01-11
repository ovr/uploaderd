package main

import "log"

func startUploader(channel chan []byte) {
	for {
		select {
		case imageBlob := <- uploadChannel:
			log.Print("[Event] New Image to Upload ", len(imageBlob));
		}
	}
}
