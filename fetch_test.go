package main

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/jbenet/go-random"
)

// Warning: these tests assume some networking capabilities... sorry

var (
	gitPath string
)

// rand strings
func init() {
	rand.Seed(time.Now().Unix())
}

// checks for the needed tools
func checkInstalled(t *testing.T) {
	var err error
	gitPath, err = exec.LookPath("git")
	checkFatal(t, err)
	out, err := exec.Command("go", "install", "github.com/cryptix/git-remote-ipfs").CombinedOutput()
	if len(out) > 0 {
		t.Log(fmt.Sprintf("%q", string(out)))
	}
	checkFatal(t, err)
	_, err = exec.LookPath("git-remote-ipfs")
	checkFatal(t, err)
}

var expectedClone = map[string]string{
	"testA":     "9417d011822b875da72221c8d188089cbfcee806",
	"hello.txt": "e2839ad2e47386d342038958fba941fc78e3780e",
	"notes":     "32ed91604b272860ec911fc2bf4ae631b7900aa8",
}

func TestClone(t *testing.T) {
	// pinned by pinbot, prepared with 'git-ipfs-rehost https://github.com/cryptix/git-remote-ipfs-testcase'
	rmDir(t, cloneAndCheckout(t, "ipfs://ipfs/QmRKt5VfMS92x77eFQFUVVmYLdLUv24gxyjS7MsMc62B1K/git-remote-ipfs-testcase", expectedClone))
}

func TestClone_unpacked(t *testing.T) {
	// pinned by pinbot, prepared with 'git-ipfs-rehost --unpack https://github.com/cryptix/git-remote-ipfs-testcase unpackedTest'
	rmDir(t, cloneAndCheckout(t, "ipfs://ipfs/QmZhuM4TxuhxbamPtWHyHYCUXfkqCkgBmWREKF2kqTLbvz/unpackedTest", expectedClone))
}

// helpers

func cloneAndCheckout(t *testing.T, repo string, expected map[string]string) (tmpDir string) {
	checkInstalled(t)
	tmpDir = mkRandTmpDir(t)
	cloneCmd := exec.Command(gitPath, "clone", repo, tmpDir)
	out, err := cloneCmd.CombinedOutput()
	t.Logf("'git clone %s %s':\n%s", repo, tmpDir, out)
	checkFatal(t, err)
	if !cloneCmd.ProcessState.Success() {
		t.Fatal("git clone failed")
	}
	hashMap(t, tmpDir, expected)
	return tmpDir
}

func checkFatal(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}

// oh well.. just some rand string
func mkRandTmpDir(t *testing.T) string {
	var buf bytes.Buffer
	for i := 0; i < 10; i++ {
		checkFatal(t, random.WriteRandomBytes(20, &buf))
		randStr := fmt.Sprintf("git-remote-ipfs-test-%x", buf.String())
		tmpDir := filepath.Join("/", os.TempDir(), randStr)
		_, err := os.Stat(tmpDir)
		if os.IsNotExist(err) {
			checkFatal(t, os.MkdirAll(tmpDir, 0700))
			t.Logf("tmpDir created: %s", tmpDir)
			return tmpDir
		}
		buf.Reset()
	}
	t.Fatal("couldnt find a tmpDir")
	return ""
}

func rmDir(t *testing.T, dir string) { checkFatal(t, os.RemoveAll(dir)) }

func hashMap(t *testing.T, dir string, files map[string]string) {
	for fname, want := range files {
		f, err := os.Open(filepath.Join(dir, fname))
		checkFatal(t, err)
		h := sha1.New()
		_, err = io.Copy(h, f)
		checkFatal(t, err)
		got := h.Sum(nil)
		if want != fmt.Sprintf("%x", got) {
			t.Errorf("hashMap: compare of %s failed\nWant: %s\nGot:  %x", fname, want, got)
		}
	}
}
