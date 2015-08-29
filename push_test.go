package main

import (
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPush(t *testing.T) {
	// $ git clone ipfs://$hash/repo.git $tmpDir
	tmpDir := cloneAndCheckout(t, "ipfs://QmNRzJ6weMUs8SpeGApfY6XZEPcVbg1PTAARFZJ2C2McJq/git-remote-ipfs-testcase", expectedClone)

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

	rmDir(t, tmpDir)
}
