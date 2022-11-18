package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/sys/unix"
)

const sharePath = "/judge/problems/"

var reloadLock sync.Mutex

func main() {
	log.SetFlags(log.Lshortfile) // timestamps are provided by systemd etc

	var addr string

	flag.StringVar(&addr, "addr", "127.1:8888", "bind address")
	flag.Parse()

	router := mux.NewRouter()
	router.Methods("GET").Path("/reload").HandlerFunc(reload)

	srv := &http.Server{
		Handler:      router,
		Addr:         addr,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}

// syncShare syncs the problem share.
func syncShare() error {
	f, err := os.Open(sharePath)
	if err != nil {
		return err
	}
	_, _, err = unix.Syscall(unix.SYS_SYNCFS, f.Fd(), 0, 0)
	if err != nil {
		return err
	}
	return nil
}

// reload handles a reload request. This acquires a mutex.
func reload(w http.ResponseWriter, r *http.Request) {
	reloadLock.Lock()
	defer reloadLock.Unlock()

	err := syncShare()
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "sync fs: %s", err)
		return
	}

	client := &http.Client{Timeout: 2 * time.Second}
	s, err := client.Post("http://localhost:9998/update/problems", "text/plain", nil)
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "reload judge: %s", err)
		return
	}
	body, err := ioutil.ReadAll(s.Body)
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "ReadAll: %s", err)
		return
	}
	if s.StatusCode != 200 {
		w.WriteHeader(500)
		fmt.Fprintf(w, "response code not 200: %d %s %s", s.StatusCode, s.Status, body)
		return
	}
	w.WriteHeader(200)
	_, _ = w.Write(body)
}
