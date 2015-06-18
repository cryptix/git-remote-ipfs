// git-remote-helper implements a git-remote helper that uses the ipfs transport
//
// ie: git clone ipfs://$some/path/to/.git
//
// see https://git-scm.com/docs/gitremote-helpers for more
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

var tmpBareRepo string

func main() {
	log.SetFlags(0)
	log.SetOutput(os.Stderr)
	log.SetPrefix("git-remote-ipfs: ")

	//log.Println("Hello", os.Environ())

	var u string
	v := len(os.Args[1:])
	switch v {
	case 2:
		log.Println("repo:", os.Args[1])
		log.Println("url:", os.Args[2])
		u = os.Args[2]

	default:
		log.Fatalf("usage: unkonw # of args: %d", v)
	}

	repoUrl, err := url.Parse(u)
	if err != nil {
		log.Fatalf("url.Parse() failed: %s", err)
	}
	log.Printf("dbg: repo url %#v", repoUrl)

	if repoUrl.Scheme != "ipfs" {
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
	log.Println("root hash: ", hash)

	tmpBareRepo = fetchRepo(hash)

	go speakGit(os.Stdin)

	select {}
}

func fetchRepo(root string) string {
	// TODO: document host format
	shell := shell.NewShell("localhost:5001")

	tmpPath := filepath.Join("/", os.TempDir(), root)
	s, err := os.Stat(tmpPath)
	if err != nil {
		if os.IsNotExist(err) {
			if err := shell.Get(root, tmpPath); err != nil {
				log.Fatalf("shell.Get(%s, %s) failed: %s", root, tmpPath, err)
			}
			log.Println("shell got:", root)
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
	r = debug.NewReadLogger("from git:", r)
	w := debug.NewWriteLogger("to git:", os.Stdout)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		text := scanner.Text()
		switch {
		case text == "capabilities":
			log.Println("caps line")
			fmt.Fprintf(w, "fetch\n\n")

		case text == "list":
			log.Println("list line")

			var b bytes.Buffer
			log.Println("tmp repo:", tmpBareRepo)
			cmd := exec.Command("git", "ls-remote", tmpBareRepo)
			cmd.Stdout = &b
			cmd.Stderr = &b

			if err := cmd.Run(); err != nil {
				log.Fatalf("git ls-remote wait error: %s", err)
			}

			log.Println("ran git ls-remote")
			tabToSpace := strings.NewReplacer("\t", " ")
			_, err := tabToSpace.WriteString(w, b.String())
			if err != nil {
				log.Fatalf("git ls-remote tabWriter error: %s", err)
			}
			fmt.Fprintln(w, "")

		case strings.HasPrefix(text, "fetch "):
			fetchSplit := strings.Split(text, " ")
			log.Printf("fetch sha1<%s> name<%s>", fetchSplit[1], fetchSplit[2])

		default:
			log.Println("default git speak:", text)
			os.Exit(1)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("stdin scanner error: %s", err)
	}
}

// shamefully taken from https://gist.github.com/tyndyll/89fbb2c2273f83a074dc
func Execute(output_buffer *bytes.Buffer, stack ...*exec.Cmd) (err error) {
	var error_buffer bytes.Buffer
	pipe_stack := make([]*io.PipeWriter, len(stack)-1)
	i := 0
	for ; i < len(stack)-1; i++ {
		stdin_pipe, stdout_pipe := io.Pipe()
		stack[i].Stdout = stdout_pipe
		stack[i].Stderr = &error_buffer
		stack[i+1].Stdin = stdin_pipe
		pipe_stack[i] = stdout_pipe
	}
	stack[i].Stdout = output_buffer
	stack[i].Stderr = &error_buffer

	if err := call(stack, pipe_stack); err != nil {
		log.Fatalln(string(error_buffer.Bytes()), err)
	}
	return err
}

func call(stack []*exec.Cmd, pipes []*io.PipeWriter) (err error) {
	if stack[0].Process == nil {
		if err = stack[0].Start(); err != nil {
			return err
		}
	}
	if len(stack) > 1 {
		if err = stack[1].Start(); err != nil {
			return err
		}
		defer func() {
			if err == nil {
				pipes[0].Close()
				err = call(stack[1:], pipes[1:])
			}
		}()
	}
	return stack[0].Wait()
}
