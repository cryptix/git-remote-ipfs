package main

import (
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/whyrusleeping/ipfs-shell"
)

func usage() {
	fmt.Fprint(os.Stderr, "usage: git-remote-ipfs <repository> [<URL>]\n")
	os.Exit(2)
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("git-remote-ipfs: ")

	var u string
	v := len(os.Args[1:])
	switch v {
	case 2:
		log.Println("remote?:", os.Args[1])
		log.Println("url:", os.Args[2])
		u = os.Args[2]

	default:
		log.Fatalf("unkonw # of args: %d", v)
	}

	repoUrl, err := url.Parse(u)
	if err != nil {
		log.Fatalf("url.Parse() failed: %s", err)
	}
	log.Printf("dbg: repo url %#v", repoUrl)

	shell := shell.NewShell("http://localhost:5001")

	shell.Get(fmt.Sprintf("/ipfs/%s/%s", repoUrl.Host, repoUrl.Path))

}
