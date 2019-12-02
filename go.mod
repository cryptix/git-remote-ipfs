module git-remote-ipfs

go 1.13

replace internal/path => ./internal/path

require (
	github.com/cryptix/exp v0.0.0-20191103140156-9da154296953
	github.com/cryptix/go v1.5.0
	github.com/ipfs/go-cid v0.0.3 // indirect
	github.com/ipfs/go-ipfs-api v0.0.2
	github.com/jbenet/go-random v0.0.0-20190219211222-123a90aedc0c
	github.com/multiformats/go-multihash v0.0.9 // indirect
	github.com/pkg/errors v0.8.1
	internal/path v0.0.0-00010101000000-000000000000
)
