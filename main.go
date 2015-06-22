// git-remote-helper implements a git-remote helper that uses the ipfs transport
//
// ie: git clone ipfs://$some/path/to/.git
//
// see https://git-scm.com/docs/gitremote-helpers for more
//
// progress: https://github.com/cryptix/git-remote-ipfs/issues/1

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cryptix/go/debug"
	"github.com/whyrusleeping/ipfs-shell"
)

const usageMsg = `usage git-remote-ipfs <repository> [<URL>]
supports ipfs://$hash/path..

TODO:
- urls like ipfs::/path/
`

func usage() {
	fmt.Fprint(os.Stderr, usageMsg)
	os.Exit(2)
}

var (
	tmpBareRepo string
)

func main() {
	log.SetFlags(0)
	log.SetOutput(os.Stderr)
	log.SetPrefix("git-remote-ipfs:")

	//log.Println("Hello", os.Environ())

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

	// since we get a proper URL, we can parse it make sure its valid
	repoUrl, err := url.Parse(u)
	if err != nil {
		log.Fatalf("url.Parse() failed: %s", err)
	}
	log.Printf("dbg: repo url %#v", repoUrl)

	if repoUrl.Scheme != "ipfs" { // ipns will have a seperate helper
		log.Fatal("only ipfs schema is supported")
	}

	// get root hash of the passed repo path
	path := fmt.Sprintf("/ipfs/%s/%s", repoUrl.Host, repoUrl.Path)
	var b bytes.Buffer
	if err := Execute(&b,
		exec.Command("ipfs", "object", "get", path),
		exec.Command("ipfs", "object", "put", "--inputenc=json"),
	); err != nil {
		log.Fatalln("root hash pipe failed", err)
	}

	hash := b.String()
	const expAdded = "added "
	if !strings.HasPrefix(hash, expAdded) {
		log.Fatal("invalid output of root-hash-pipe, expected: %s got: %s", expAdded, hash)
	}
	hash = hash[len(expAdded):]
	log.Println("DEBUG: root hash: ", hash)

	tmpBareRepo = fetchFullBareRepo(hash)

	go speakGit(os.Stdin)

	select {} // block indefinetly - until stdin closes most likly
}

func fetchFullBareRepo(root string) string {
	// TODO: document host format
	shell := shell.NewShell("localhost:5001")

	tmpPath := filepath.Join("/", os.TempDir(), root)
	s, err := os.Stat(tmpPath)
	if err != nil {
		if os.IsNotExist(err) {
			if err := shell.Get(root, tmpPath); err != nil {
				log.Fatalf("shell.Get(%s, %s) failed: %s", root, tmpPath, err)
			}
			log.Println("DEBUG: shell got:", root)
			return tmpPath
		}
		log.Fatalf("stat err: %s", err)
	}
	if !s.IsDir() {
		log.Fatalf("please delete %s")
	}
	return tmpPath
}

// speakGit acts like a git-remote-helper
// see this for more: https://www.kernel.org/pub/software/scm/git/docs/gitremote-helpers.html
func speakGit(r io.Reader) {
	r = debug.NewReadLogger("git>>", r)
	w := debug.NewWriteLogger("git<<", os.Stdout)

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		text := scanner.Text()
		switch {

		case text == "capabilities":
			log.Println("DEBUG: got caps line")
			fmt.Fprintf(w, "fetch\n\n")

		case text == "list":
			log.Println("DEBUG: got list line")

			var b bytes.Buffer
			log.Println("DEBUG: tmp repo:", tmpBareRepo)
			cmd := exec.Command("git", "ls-remote", tmpBareRepo)
			cmd.Stdout = &b
			cmd.Stderr = &b

			if err := cmd.Run(); err != nil {
				log.Fatalf("git ls-remote Run error: %s", err)
			}

			log.Println("DEBUG: ran git ls-remote")
			// convert tabs to spaces
			tabToSpace := strings.NewReplacer("\t", " ")
			_, err := tabToSpace.WriteString(w, b.String())
			if err != nil {
				log.Fatalf("git ls-remote tab conversion error: %s", err)
			}
			fmt.Fprintln(w, "")

		case strings.HasPrefix(text, "fetch "):
			fetchSplit := strings.Split(text, " ")
			if len(fetchSplit) < 2 {
				log.Printf("malformed 'fetch' command. %q", text)
			}
			log.Printf("DEBUG: fetch sha1<%s> name<%s>", fetchSplit[1], fetchSplit[2])

		default:
			log.Printf("DEBUG: default git speak: %q\n", text)
			os.Exit(1)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("stdin scanner error: %s", err)
	}
}
