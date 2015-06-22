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
	"gopkg.in/errgo.v1"
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
	thisGitRepo string
)

func main() {
	log.SetFlags(0)
	log.SetOutput(os.Stderr)
	log.SetPrefix("git-remote-ipfs:")

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
			err := fetchObject(fetchSplit[1])
			if err == nil {
				log.Println("fetchObject() worked")
				//fmt.Fprintln(w, "")
				continue
			}
			log.Println("method1 failed:", err)

			err = fetchPackedObject(fetchSplit[1])
			if err == nil {
				log.Println("fetchPackedObject() worked")
				continue
			}
			log.Println("method2 failed:", err)
			os.Exit(1)

		case text == "":
			log.Println("DEBUG: got empty line (end of fetch batch?)")
			fmt.Fprintln(w, "")

		default:
			log.Printf("DEBUG: default git speak: %q\n", text)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("stdin scanner error: %s", err)
	}

	log.Println("speakGit: exited read loop")
	os.Exit(0)
}

// "fetch $sha1 $ref" method 1
//   - look for it in ".git/objects/substr($sha1, 0, 2)/substr($sha, 2)"
//   - if found, download it and put it in place. (there may be a command for this)
//   - done \o/
func fetchObject(sha1 string) error {
	p := filepath.Join(tmpBareRepo, "objects", sha1[:2], sha1[2:])
	packF, err := os.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return err
		}
		return errgo.Notef(err, "fetchObject: sha1<%s> open() failed", sha1)
	}

	if thisGitRepo == "" {
		log.Fatal("GIT_DIR is unset. how can this be?")
	}

	// TODO: didnt find a git command to impot loose blob objects

	fanOutDir := filepath.Join(thisGitRepo, "objects", sha1[:2])
	log.Printf("DEBUG: fan-out dir:%q gitDir: %q", fanOutDir, thisGitRepo)
	if err := os.MkdirAll(fanOutDir, 0700); err != nil { // TODO: not sure about the mode here
		return errgo.Notef(err, "fetchObject: sha1<%s> could not create fan-out dir", sha1)
	}

	newPackF, err := os.Create(filepath.Join(fanOutDir, sha1[2:]))
	if err != nil {
		return errgo.Notef(err, "fetchObject: sha1<%s> os.Create(newPackF) failed", sha1)
	}

	if _, err := io.Copy(newPackF, packF); err != nil {
		return errgo.Notef(err, "fetchObject: sha1<%s> io.Copy(new, old) failed", sha1)
	}

	if err := packF.Close(); err != nil {
		return errgo.Notef(err, "fetchObject: sha1<%s> close(packF) failed", sha1)
	}

	if err := newPackF.Close(); err != nil {
		return errgo.Notef(err, "fetchObject: sha1<%s> close(newPackF) failed", sha1)
	}

	return nil
}

// "fetch $sha1 $ref" method 2
//   - look for it in packfiles by fetching ".git/objects/pack/*.idx"
//     and looking at each idx with cat <idx> | git show-index  (alternatively can learn to read the format in go)
//   - if found in an <idx>, download the relevant .pack file,
//     and feed it into `git index-pack --stdin --fix-thin` which will put it into place.
//   - done \o/
func fetchPackedObject(sha1 string) error {
	// search for all index files
	p := filepath.Join(tmpBareRepo, "objects", "pack", "*.idx")
	indexes, err := filepath.Glob(p)
	if err != nil {
		return errgo.Notef(err, "fetchPackedObject: glob(%q) failed", p)
	}

	if len(indexes) == 0 {
		return errgo.New("fetchPackedObject: no idx files found")
	}

	for _, idx := range indexes {
		idxF, err := os.Open(idx)
		if err != nil {
			return errgo.Notef(err, "fetchPackedObject: idx<%s> open() failed", sha1)
		}
		defer idxF.Close()

		// using external git show-index < idxF for now
		// TODO: parse index file in go to make this portable
		var b bytes.Buffer
		showIdx := exec.Command("git", "show-index")
		showIdx.Stdin = idxF
		showIdx.Stdout = &b

		if err := showIdx.Run(); err != nil {
			return errgo.Notef(err, "fetchPackedObject: idx<%s> show-index start failed", sha1)
		}

		if !strings.Contains(b.String(), sha1) {
			// sha1 not in index, next idx file
			continue
		}

		if err := idxF.Close(); err != nil {
			return errgo.Notef(err, "fetchPackedObject: idx<%s> idxF.close() failed", sha1)
		}

		// we found an index with our hash inside
		pack := strings.Replace(idx, ".idx", ".pack", 1)
		packF, err := os.Open(pack)
		if err != nil {
			return errgo.Notef(err, "fetchPackedObject: pack<%s> open() failed", sha1)
		}

		b.Reset()
		unpackIdx := exec.Command("git", "unpack-objects")
		unpackIdx.Dir = thisGitRepo // GIT_DIR
		unpackIdx.Stdin = packF
		unpackIdx.Stdout = &b

		if err := unpackIdx.Run(); err != nil {
			return errgo.Notef(err, "fetchPackedObject: pack<%s> 'git unpack-objects' failed\nOutput: %s", sha1, b.String())
		}

		if err := packF.Close(); err != nil {
			return errgo.Notef(err, "fetchPackedObject: pack<%s> packF.close() failed", sha1)
		}
		// found and unpacked - done
		return nil
	}

	return errgo.Newf("did not find sha1<%s> in %d index files", sha1, len(indexes))
}
