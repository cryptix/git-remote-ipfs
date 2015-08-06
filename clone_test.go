package main

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/jbenet/go-random"
	"github.com/kylelemons/godebug/diff"
)

// Warning: these tests assume some networking capabilities... sorry

var (
	ipfsPath string
	gitPath  string
)

func init() {
	rand.Seed(time.Now().Unix())
}

// checks for the needed tools
func checkInstalled(t *testing.T) {
	var err error
	ipfsPath, err = exec.LookPath("ipfs")
	if err != nil {
		t.Fatal("ipfs is not installed")
	}
	gitPath, err = exec.LookPath("git")
	if err != nil {
		t.Fatal("git is not installed")
	}
	_, err = exec.LookPath("git-remote-ipfs")
	if err != nil {
		t.Fatal("git-remote-ipfs is not installed")
	}

	// check for daemon... maybe need to init ipfs
	if os.Getenv("WERCKER_STEP_NAME") != "" {
		if err := exec.Command(ipfsPath, "init").Run(); err != nil {
			t.Fatal("ipfs init failed")
		}
	}
}

func TestClone(t *testing.T) {
	checkInstalled(t)

	// oh well.. just some rand string
	var buf bytes.Buffer
	if err := random.WriteRandomBytes(20, &buf); err != nil {
		t.Fatalf("get random str: %s", err)
	}
	randStr := fmt.Sprintf("git-remote-ipfs-test-%x", buf.String())

	tmpDir := filepath.Join("/", os.TempDir(), randStr)
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		t.Fatalf("mkdirAll(%q): %s", tmpDir, err)
	}
	t.Logf("dbg: created %s", tmpDir)

	// pinned by pinbot, prepared with 'git-ipfs-rehost https://github.com/cryptix/git-remote-ipfs-testcase'
	buf.Reset()
	cloneCmd := exec.Command(gitPath, "clone", "ipfs://QmS5Vauz2G6DVP7NEetJBcHDUNPTRt34D6evNiwrp7Gmsk/git-remote-ipfs-testcase", tmpDir)
	cloneCmd.Stdout = &buf
	cloneCmd.Stderr = &buf
	if err := cloneCmd.Run(); err != nil { // exit status 0?
		t.Log(buf.String())
		t.Fatalf("git clone ipfs:// failed: %s", err)
	}

	// TODO(cryptix): maybe just use an md5 walker?
	buf.Reset()
	addCmd := exec.Command(ipfsPath, "add", "-q", "-r", tmpDir)
	addCmd.Stdout = &buf
	if err := addCmd.Run(); err != nil {
		t.Fatalf("ipfs add for comparison failed: %s", err)
	}

	// compare cloned hashes against expected ones
	const expected = `QmSS1VNgmPW8yFxYoHEUHDEEz8FYBucMqN8xY92Y9pGq26
QmSKoJo4VSso89bhbiTnVsgC7jKyqdBcB5GYCozYFp7fNs
QmaSPaHCETQmfLo7SigbaqsCHcZgivcWALMhWVnxEaNutj
QmPiW5xxfhVA2YaVoqLXsbMvdLMuVBNL68NPL57aWCAV8X
`
	if diff := diff.Diff(expected, buf.String()); diff != "" {
		t.Fatal(diff)
	}

	if err := os.RemoveAll(tmpDir); err != nil { // cleanup tmpDir
		t.Error(err)
	}
}
