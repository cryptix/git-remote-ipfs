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

https://git-scm.com/book/en/v2/Git-Internals-Transfer-Protocols
*/
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/cryptix/go/debug"
	"github.com/cryptix/go/logging"
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
	ref2hash = make(map[string]string)

	ipfsShell    = shell.NewShell("localhost:5001")
	ipfsRepoPath string
	thisGitRepo  string
	errc         chan<- error
	log          = logging.Logger("git-remote-ipfs")
)

func main() {
	// logging
	logging.SetupLogging(nil)

	// env var and arguments
	thisGitRepo = os.Getenv("GIT_DIR")
	if thisGitRepo == "" {
		log.Fatal("could not get GIT_DIR env var")
	}
	if thisGitRepo == ".git" {
		cwd, err := os.Getwd()
		logging.CheckFatal(err)
		thisGitRepo = filepath.Join(cwd, ".git")
	}
	log.Debug("GIT_DIR=", thisGitRepo)

	var u string // repo url
	v := len(os.Args[1:])
	switch v {
	case 2:
		log.Debug("repo:", os.Args[1])
		log.Debug("url:", os.Args[2])
		u = os.Args[2]
	default:
		log.Fatalf("usage: unknown # of args: %d\n%v", v, os.Args[1:])
	}

	// parse passed URL
	repoURL, err := url.Parse(u)
	if err != nil {
		log.Fatalf("url.Parse() failed: %s", err)
	}
	if repoURL.Scheme != "ipfs" { // ipns will have a seperate helper(?)
		log.Fatal("only ipfs schema is supported")
	}
	ipfsRepoPath = fmt.Sprintf("/ipfs/%s/%s", repoURL.Host, repoURL.Path)

	// interrupt / error handling
	go func() {
		if err := interrupt(); err != nil {
			log.Fatal("interrupted:", err)
		}
	}()

	if err := speakGit(os.Stdin, os.Stdout); err != nil {
		log.Fatal("speakGit failed:", err)
	}
}

func getRefs(forPush bool) error {
	refsCat, err := ipfsShell.Cat(filepath.Join(ipfsRepoPath, "info", "refs"))
	if err != nil {
		return errgo.Notef(err, "failed to cat info/refs from %s", ipfsRepoPath)
	}
	s := bufio.NewScanner(refsCat)
	for s.Scan() {
		hashRef := strings.Split(s.Text(), "\t")
		if len(hashRef) != 2 {
			return errgo.Newf("processing info/refs: what is this: %v", hashRef)
		}
		ref2hash[hashRef[1]] = hashRef[0]
		log.WithField("ref", hashRef[1]).WithField("sha1", hashRef[0]).Debug("got ref")
	}
	if err := s.Err(); err != nil {
		return errgo.Notef(err, "ipfs.Cat(info/refs) scanner error")
	}
	return nil
}

func getHeadRef() (string, error) {
	headCat, err := ipfsShell.Cat(filepath.Join(ipfsRepoPath, "HEAD"))
	if err != nil {
		return "", errgo.Notef(err, "failed to cat HEAD from %s", ipfsRepoPath)
	}
	head, err := ioutil.ReadAll(headCat)
	if err != nil {
		return "", errgo.Notef(err, "failed to readAll HEAD from %s", ipfsRepoPath)
	}
	if !bytes.HasPrefix(head, []byte("ref: ")) {
		return "", errgo.Newf("illegal HEAD file from %s: %q", ipfsRepoPath, head)
	}
	headRef := string(bytes.TrimSpace(head[5:]))
	headHash, ok := ref2hash[headRef]
	if !ok {
		// use first hash in map?..
		return "", errgo.Newf("unknown HEAD reference %q", headRef)
	}
	log.WithField("ref", headRef).WithField("sha1", headHash).Debug("got HEAD ref")
	return headHash, nil
}

// speakGit acts like a git-remote-helper
// see this for more: https://www.kernel.org/pub/software/scm/git/docs/gitremote-helpers.html
func speakGit(r io.Reader, w io.Writer) error {
	debugLog := logging.Logger("git")
	r = debug.NewReadLogrus(debugLog, r)
	w = debug.NewWriteLogrus(debugLog, w)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		text := scanner.Text()
		switch {

		case text == "capabilities":
			fmt.Fprintln(w, "fetch")
			fmt.Fprintln(w, "push")
			fmt.Fprintln(w, "")

		case strings.HasPrefix(text, "list"):
			log.Debug("got list line")

			if err := getRefs(false); err != nil {
				return err
			}
			//TODO: alternativly iterate over the refs directory like git-remote-dropbox

			head, err := getHeadRef()
			if err != nil {
				return err
			}
			// output
			fmt.Fprintf(w, "%s HEAD\n", head)
			for ref, hash := range ref2hash {
				fmt.Fprintf(w, "%s %s\n", hash, ref)
			}
			fmt.Fprintln(w, "")

		case strings.HasPrefix(text, "fetch "):
			fetchSplit := strings.Split(text, " ")
			if len(fetchSplit) < 2 {
				return errgo.Newf("malformed 'fetch' command. %q", text)
			}
			f := map[string]interface{}{
				"sha1": fetchSplit[1],
				"name": fetchSplit[2],
			}
			err := fetchObject(fetchSplit[1])
			if err == nil {
				log.WithFields(f).Info("fetched loose")
				fmt.Fprintln(w, "")
				continue
			}
			log.WithFields(f).WithField("err", err).Debug("fetchLooseObject failed, trying packed...")
			err = fetchPackedObject(fetchSplit[1])
			if err != nil {
				return errgo.Notef(err, "fetchPackedObject() failed")
			}
			log.WithFields(f).Info("fetched packed")
			fmt.Fprintln(w, "")

		case text == "":
			// TODO(cryptix): count fetches and track them
			log.Debug("got empty line (end of fetch batch?)")
			fmt.Fprintln(w, "")
			return nil

		default:
			return errgo.Newf("Error: default git speak: %q", text)
		}
	}

	if err := scanner.Err(); err != nil {
		return errgo.Notef(err, "scanner.Err()")
	}

	log.Info("speakGit: exited read loop")
	return nil
}
