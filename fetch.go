package main

import (
	"bytes"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/errgo.v1"
)

// "fetch $sha1 $ref" method 1
//   - look for it in ".git/objects/substr($sha1, 0, 2)/substr($sha, 2)"
//   - if found, download it and put it in place. (there may be a command for this)
//   - done \o/
func fetchObject(sha1 string) error {
	return errgo.Newf("TODO: unsupported - please see issue #1")
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
