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
	if err != nil {
		t.Fatal("git is not installed")
	}
	_, err = exec.LookPath("git-remote-ipfs")
	if err != nil {
		t.Log("git-remote-ipfs is not installed")
		if out, err := exec.Command("go", "install", "github.com/cryptix/git-remote-ipfs").CombinedOutput(); err != nil {
			t.Log(fmt.Sprintf("%q", string(out)))
			t.Fatal("go install failed:", err)
		}
	}
}

// oh well.. just some rand string
func mkRandTmpDir(t *testing.T) string {
	var buf bytes.Buffer
	for i := 0; i < 10; i++ {
		if err := random.WriteRandomBytes(20, &buf); err != nil {
			t.Fatalf("get random str: %s", err)
		}
		randStr := fmt.Sprintf("git-remote-ipfs-test-%x", buf.String())
		tmpDir := filepath.Join("/", os.TempDir(), randStr)
		_, err := os.Stat(tmpDir)
		if os.IsNotExist(err) {
			if err := os.MkdirAll(tmpDir, 0700); err != nil {
				t.Fatalf("mkdirAll(%q): %s", tmpDir, err)
			}
			t.Logf("dbg: created %s", tmpDir)
			return tmpDir
		}
		buf.Reset()
	}
	t.Fatal("couldnt find a tmpDir")
	return ""
}

var expectedClone = map[string][]byte{
	"testA":     []byte("\x94\x17\xd0\x11\x82+\x87]\xa7\"!\xc8ш\b\x9c\xbf\xce\xe8\x06"),
	"hello.txt": []byte("⃚\xd2\xe4s\x86\xd3B\x03\x89X\xfb\xa9A\xfcx\xe3x\x0e"),
	"notes":     []byte("2\xed\x91`K'(`\xec\x91\x1f¿J\xe61\xb7\x90\n\xa8"),
}

func TestClone(t *testing.T) {
	// pinned by pinbot, prepared with 'git-ipfs-rehost https://github.com/cryptix/git-remote-ipfs-testcase'
	cloneAndCheckout(t, "ipfs://QmNRzJ6weMUs8SpeGApfY6XZEPcVbg1PTAARFZJ2C2McJq/git-remote-ipfs-testcase", expectedClone)
}

func TestClone_unpacked(t *testing.T) {
	// pinned by pinbot, prepared with 'git-ipfs-rehost --unpack https://github.com/cryptix/git-remote-ipfs-testcase unpackedTest'
	cloneAndCheckout(t, "ipfs://QmYFpZJs82hLTyEpwkzVpaXGUabVVwiT8yrd6TK81XnoGB/unpackedTest", expectedClone)
}

func cloneAndCheckout(t *testing.T, repo string, expected map[string][]byte) {
	checkInstalled(t)

	tmpDir := mkRandTmpDir(t)

	var buf bytes.Buffer
	cloneCmd := exec.Command(gitPath, "clone", repo, tmpDir)
	cloneCmd.Stdout = &buf
	cloneCmd.Stderr = &buf
	err := cloneCmd.Run()
	t.Log(buf.String())
	if err != nil { // exit status 0?
		t.Fatalf("git clone ipfs:// failed: %s\nOutput:%s", err, buf.String())
	}

	hashMap(t, tmpDir, expected)

	if err := os.RemoveAll(tmpDir); err != nil { // cleanup tmpDir
		t.Error(err)
	}
}

func hashMap(t *testing.T, dir string, files map[string][]byte) {
	for fname, want := range files {
		f, err := os.Open(filepath.Join(dir, fname))
		if err != nil {
			t.Fatalf("hashMap open err: %s", err)
		}
		h := sha1.New()

		if _, err := io.Copy(h, f); err != nil {
			t.Fatalf("hashMap copy err: %s", err)
		}
		got := h.Sum(nil)
		if bytes.Compare(want, got) != 0 {
			t.Errorf("hashMap: compare of %s failed\nWant: %q\nGot:  %q", fname, want, got)
		}

	}
}
