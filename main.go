/*
git-remote-helper implements a git-remote helper that uses the ipfs transport.

TODO

Currently assumes a IPFS Daemon at localhost:5001

Not completed: Push, IPNS, URLs like ipfs::path/.., embedded IPFS node

...

 $ git clone ipfs://$hash/repo.git
 $ cd repo && make $stuff
 $ git commit -a -m 'done!'
 $ git push origin
 => clone-able as ipfs://$newHash/repo.git

Links

https://ipfs.io

https://github.com/whyrusleeping/git-ipfs-rehost

https://git-scm.com/docs/gitremote-helpers
*/
package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/cryptix/go/debug"
	"github.com/ipfs/go-ipfs-shell"
	"gopkg.in/errgo.v1"
)

const usageMsg = `usage git-remote-ipfs <repository> [<URL>]
supports ipfs://$hash/path..

`

func usage() {
	fmt.Fprint(os.Stderr, usageMsg)
	os.Exit(2)
}

var (
	ipfsShell    = shell.NewShell("localhost:5001")
	ipfsRepoPath string
	thisGitRepo  string
	errc         chan<- error
)

func main() {
	// logging
	log.SetFlags(0)
	log.SetOutput(os.Stderr)
	log.SetPrefix("git-remote-ipfs:")

	// env var and arguments
	thisGitRepo := os.Getenv("GIT_DIR")
	if thisGitRepo == "" {
		log.Fatal("could not get GIT_DIR env var")
	}
	log.Println("GIT_DIR=", thisGitRepo)

	var u string // repo url
	v := len(os.Args[1:])
	switch v {
	case 2:
		log.Println("repo:", os.Args[1])
		log.Println("url:", os.Args[2])
		u = os.Args[2]
	default:
		log.Fatalf("usage: unknown # of args: %d\n%v", v, os.Args[1:])
	}

	// parse passed URL
	repoUrl, err := url.Parse(u)
	if err != nil {
		log.Fatalf("url.Parse() failed: %s", err)
	}
	if repoUrl.Scheme != "ipfs" { // ipns will have a seperate helper(?)
		log.Fatal("only ipfs schema is supported")
	}
	ipfsRepoPath = fmt.Sprintf("/ipfs/%s/%s", repoUrl.Host, repoUrl.Path)

	// interrupt / error handling
	ec := make(chan error)
	errc = ec
	go func() {
		errc <- interrupt()
	}()

	go speakGit(os.Stdin, os.Stdout)

	log.Println("closing error:", <-ec)
}

// speakGit acts like a git-remote-helper
// see this for more: https://www.kernel.org/pub/software/scm/git/docs/gitremote-helpers.html
func speakGit(r io.Reader, w io.Writer) {
	r = debug.NewReadLogger("git>>", r)
	w = debug.NewWriteLogger("git<<", w)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		text := scanner.Text()
		switch {

		case text == "capabilities":
			log.Println("DEBUG: got caps line")
			fmt.Fprintln(w, "fetch")
			fmt.Fprintln(w, "push")
			fmt.Fprintln(w, "")

		case strings.HasPrefix(text, "list"):
			log.Println("DEBUG: got list line")
			refsCat, err := ipfsShell.Cat(filepath.Join(ipfsRepoPath, "info", "refs"))
			if err != nil {
				errc <- errgo.Notef(err, "failed to cat info/refs from %s", ipfsRepoPath)
				return
			}
			refs, err := ioutil.ReadAll(refsCat)
			if err != nil {
				errc <- errgo.Notef(err, "failed to readAll info/refs from %s", ipfsRepoPath)
				return
			}
			tabToSpace := strings.NewReplacer("\t", " ")
			_, err = tabToSpace.WriteString(w, string(refs))
			if err != nil {
				errc <- errgo.Notef(err, "git list response tab>space conversion failed")
				return
			}

			fmt.Fprintln(w, "")

		case strings.HasPrefix(text, "fetch "):
			fetchSplit := strings.Split(text, " ")
			if len(fetchSplit) < 2 {
				log.Printf("malformed 'fetch' command. %q", text)
			}
			log.Printf("fetch sha1<%s> name<%s>", fetchSplit[1], fetchSplit[2])
			err := fetchObject(fetchSplit[1])
			if err == nil {
				//log.Println("fetchObject() worked")
				//fmt.Fprintln(w, "")
				continue
			}
			log.Println("method1 failed:", err)
			err = fetchPackedObject(fetchSplit[1])
			if err != nil {
				errc <- errgo.Notef(err, "fetchPackedObject() failed")
				return
			}

		case text == "":
			log.Println("DEBUG: got empty line (end of fetch batch?)")
			fmt.Fprintln(w, "")
			fmt.Fprintln(w, "")
			os.Exit(0)

		default:
			errc <- errgo.Newf("Error: default git speak: %q", text)
			return
		}
	}

	if err := scanner.Err(); err != nil {
		errc <- errgo.Notef(err, "scanner.Err()")
		return
	}

	log.Println("speakGit: exited read loop")
	errc <- nil
}
