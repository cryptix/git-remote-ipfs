package git

import (
	"crypto/sha1"
	"os"
	"testing"
	"time"

	"github.com/cheekybits/is"
	"github.com/kylelemons/godebug/diff"
)

func TestUnpackObject_blob(t *testing.T) {
	is := is.New(t)
	tcases := []struct {
		Fname  string
		Object *Object
	}{

		// blobs
		{
			Fname: "tests/blob/1f7a7a472abf3dd9643fd615f6da379c4acb3e3a",
			Object: &Object{
				Type: BlobT,
				Size: 10,
				blob: newBlob([]byte("version 2\n")),
			},
		},
		{
			Fname: "tests/blob/d670460b4b4aece5915caf5c68d12f560a9fe3e4",
			Object: &Object{
				Type: BlobT,
				Size: 13,
				blob: newBlob([]byte("test content\n")),
			},
		},

		// trees
		{
			Fname: "tests/tree/3c4e9cd789d88d8d89c1073707c3585e41b0e614",
			Object: &Object{Type: TreeT, Size: 101,
				tree: []Tree{
					{"40000", "bak", shaFromStr("\xd82\x9f\xc1̓\x87\x80\xffݟ\x94\xe0\xd3d\xe0\xeat\xf5y")},
					{"100644", "new.txt", shaFromStr("\xfaI\xb0w\x97#\x91\xadX\x03pP\xf2\xa7_t\xe3g\x1e\x92")},
					{"100644", "test.txt", shaFromStr("\x1fzzG*\xbf=\xd9d?\xd6\x15\xf6\xda7\x9cJ\xcb>:")},
				},
			},
		},
		{
			Fname: "tests/tree/0155eb4229851634a0f03eb265b69f5a2d56f341",
			Object: &Object{Type: TreeT, Size: 71,
				tree: []Tree{
					{"100644", "new.txt", shaFromStr("\xfaI\xb0w\x97#\x91\xadX\x03pP\xf2\xa7_t\xe3g\x1e\x92")},
					{"100644", "test.txt", shaFromStr("\x1fzzG*\xbf=\xd9d?\xd6\x15\xf6\xda7\x9cJ\xcb>:")},
				},
			},
		},

		// commit
		{
			Fname: "tests/commit/de70159e4a5842aed0aae9380e3006e909c8feb4",
			Object: &Object{Type: CommitT, Size: 165,
				commit: &Commit{
					Tree:      "d8329fc1cc938780ffdd9f94e0d364e0ea74f579",
					Author:    &Stamp{Name: "Henry", Email: "cryptix@riseup.net", When: time.Unix(1438988455, 0)},
					Committer: &Stamp{Name: "Henry", Email: "cryptix@riseup.net", When: time.Unix(1438988455, 0)},
					Message:   "first commit",
				},
			},
		},
		{
			Fname: "tests/commit/ad8fdc888c6f6caed63af0fb08484901e4e7e41e",
			Object: &Object{Type: CommitT, Size: 165,
				commit: &Commit{
					Tree:      "d8329fc1cc938780ffdd9f94e0d364e0ea74f579",
					Author:    &Stamp{Name: "Henry", Email: "cryptix@riseup.net", When: time.Unix(1438995813, 0)},
					Committer: &Stamp{Name: "Henry", Email: "cryptix@riseup.net", When: time.Unix(1438995813, 0)},
					Message:   "first commit",
				},
			},
			// TODO(cryptix): add example with Parent
		},
	}

	for _, tc := range tcases {
		f, err := os.Open(tc.Fname)
		is.Nil(err)
		obj, err := DecodeObject(f)
		is.Nil(err)
		diff := diff.Diff(obj.String(), tc.Object.String())
		is.Equal(diff, "")
		is.Nil(f.Close())
	}
}

func shaFromStr(str string) [sha1.Size]byte {
	var sha [sha1.Size]byte
	if len(str) != 20 {
		panic("illegal input")
	}
	var convert []byte = []byte(str)
	copy(sha[:], convert)
	return sha
}
