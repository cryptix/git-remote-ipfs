package main

import (
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPush(t *testing.T) {
	// $ git clone ipfs://ipfs/$hash/repo.git $tmpDir
	startURL := "ipfs://ipfs/QmYFpZJs82hLTyEpwkzVpaXGUabVVwiT8yrd6TK81XnoGB/unpackedTest"
	tmpDir := cloneAndCheckout(t, startURL, expectedClone)

	// $ cd repo && make $stuff
	checkFatal(t, ioutil.WriteFile(filepath.Join(tmpDir, "newFile"), []byte("Hello From Test"), 0700))
	cmd := exec.Command(gitPath, "add", "newFile")
	cmd.Dir = tmpDir
	out, err := cmd.CombinedOutput()
	t.Log("git add out: ", string(out))
	checkFatal(t, err)

	// $ git commit -a -m 'done!'
	cmd = exec.Command(gitPath, "commit", "-m", "Test Add newFile Commit")
	cmd.Dir = tmpDir
	out, err = cmd.CombinedOutput()
	t.Log("git commit out: ", string(out))
	checkFatal(t, err)

	// $ git push origin
	cmd = exec.Command(gitPath, "push", "origin")
	cmd.Dir = tmpDir
	out, err = cmd.CombinedOutput()
	t.Log("git push out: ", string(out))
	checkFatal(t, err)

	cmd = exec.Command(gitPath, "config", "--get", "remote.origin.url")
	cmd.Dir = tmpDir
	out, err = cmd.CombinedOutput()
	checkFatal(t, err)
	newURL := strings.TrimSpace(string(out))
	if newURL == startURL {
		t.Fatalf("remote url wasn't updated. is:%q", newURL)
	}
	rmDir(t, tmpDir)

	var expectedClone = map[string]string{
		"testA":     "9417d011822b875da72221c8d188089cbfcee806",
		"hello.txt": "e2839ad2e47386d342038958fba941fc78e3780e",
		"notes":     "32ed91604b272860ec911fc2bf4ae631b7900aa8",
		"newFile":   "cc7aae22f2d4301b6006e5f26e28b63579b61072",
	}
	rmDir(t, cloneAndCheckout(t, newURL, expectedClone))
}
