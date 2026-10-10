package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"torrent-cli/pkg/engine"
	"torrent-cli/pkg/vfs"
	"torrent-cli/pkg/xdg"
)

func main() {
	mountDir, err := xdg.RuntimeDir()
	if err != nil {
		log.Fatalf("error resolving runtime dir: %v", err)
	}

	eng, err := engine.New("/home/leonardo/testes")
	if err != nil {
		log.Fatalf("error starting engine: %v", err)
	}
	defer eng.Close()
	
	server, err := vfs.Mount(mountDir, eng)
	if err != nil {
		log.Fatalf("error mounting FUSE: %v", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	
	fmt.Printf("9td running! FUSE mounted at %s\n", mountDir)
	fmt.Println("Press Ctrl+C to stop")

	<-sigChan
	fmt.Println("unmounting FUSE and finishing")
	server.Unmount()
}
