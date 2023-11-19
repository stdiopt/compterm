package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/kr/pty"
	"golang.org/x/term"
	"nhooyr.io/websocket"
)

type termIO struct {
}

var (
	termio      = termIO{}
	connections []*websocket.Conn
	connMutex   sync.Mutex
	writeWSChan = make(chan []byte, 1024)
)

// appendToOutFile append bytes to out.txt file
func appendToOutFile(p []byte) {
	f, err := os.OpenFile("out.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("error opening file: %s\r\n", err)
	}
	defer f.Close()

	if _, err := f.Write(p); err != nil {
		log.Fatalf("error writing to file: %s\r\n", err)
	}
}

func writeWSLoop() {
	for {
		select {
		case msg := <-writeWSChan:
			writeAllWS(msg)
		}
	}
}

func writeAllWS(msg []byte) {
	for _, c := range connections {
		err := c.Write(context.Background(), websocket.MessageText, msg)
		if err != nil {
			if websocket.CloseStatus(err) != websocket.StatusNormalClosure {
				log.Printf("error writing to websocket: %s, %v\r\n", err, websocket.CloseStatus(err)) // TODO: send to file, not the screen
			}
			removeConnection(c) // TODO: is this safe?
		}
	}
}

func (o termIO) Write(p []byte) (n int, err error) {

	// append to out.txt file
	appendToOutFile(p)

	n, err = os.Stdout.Write(p)

	// write to websocket
	writeWSChan <- p

	return
}

func runCmd() {
	c := exec.Command(os.Args[1], os.Args[2:]...)

	// Start the command with a pty.
	ptmx, err := pty.Start(c)
	if err != nil {
		log.Fatalf("error starting pty: %s\r\n", err)
	}
	// Make sure to close the pty at the end.
	defer func() { _ = ptmx.Close() }() // Best effort.

	// Handle signals
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH, syscall.SIGTERM, os.Interrupt)
	go func() {
		for caux := range ch {
			switch caux {
			case syscall.SIGWINCH:
				// Update window size.
				_ = pty.InheritSize(os.Stdin, ptmx)

				// Send clear scape sequence to the pty them send the size of the terminal to the websocket.
				clear := "\033[H\033[2J\033[3J\033[;H\033[0m"
				sizeWidth, sizeHeight, err := term.GetSize(int(os.Stdin.Fd()))
				if err != nil {
					log.Fatalf("error getting size: %s\r\n", err)
				}
				writeWSChan <- []byte(clear)
				writeWSChan <- []byte(fmt.Sprintf("\033[8;%d;%dt", sizeHeight, sizeWidth))
			case syscall.SIGTERM, os.Interrupt:
				removeAllConnections()
				os.Exit(0)
			}
		}
	}()
	ch <- syscall.SIGWINCH // Initial resize.

	// Set stdin in raw mode.
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		log.Fatalf("error making raw: %s\r\n", err)
	}
	restoreTerm := func() {
		_ = term.Restore(int(os.Stdin.Fd()), oldState)
	}
	defer restoreTerm()

	// Copy stdin to the pty and the pty to stdout.
	go func() { _, _ = io.Copy(ptmx, os.Stdin) }()
	_, _ = io.Copy(termio, ptmx)

	// Close the stdin pipe.
	_ = ptmx.Close()

	// Wait for the command to finish.
	err = c.Wait()
	if err != nil {
		log.Fatalf("error waiting for command: %s\r\n", err)
	}

	// Close the websocket connections
	removeAllConnections()

	restoreTerm()

	os.Exit(0)
}

func removeAllConnections() {
	connMutex.Lock()
	defer connMutex.Unlock()

	for _, conn := range connections {
		err := conn.Close(websocket.StatusNormalClosure, "server shutdown")
		if err != nil {
			log.Printf("error closing websocket: %s\r\n", err)
		}
	}
	connections = []*websocket.Conn{}
}

func removeConnection(c *websocket.Conn) {
	connMutex.Lock()
	defer connMutex.Unlock()

	for i, conn := range connections {
		if conn == c {
			connections = append(connections[:i], connections[i+1:]...)
			break
		}
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, %q", r.URL.Path)
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	connMutex.Lock()
	connections = append(connections, c)
	connMutex.Unlock()
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	go runCmd()
	go writeWSLoop()

	mux := http.NewServeMux()

	mux.HandleFunc("/ws", wsHandler)
	mux.HandleFunc("/", homeHandler)

	s := &http.Server{
		Handler:        mux,
		Addr:           fmt.Sprintf(":%d", 8080),
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   5 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	log.Printf("Listening on port %d\n", 8080)
	log.Fatal(s.ListenAndServe())

}
