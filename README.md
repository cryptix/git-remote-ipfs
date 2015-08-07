# git-remote-ipfs

[![wercker status](https://app.wercker.com/status/3749e6687bf42f3cfe6114fd8d3419c1/m "wercker status")](https://app.wercker.com/project/bykey/3749e6687bf42f3cfe6114fd8d3419c1)

See [![GoDoc](https://godoc.org/github.com/cryptix/git-remote-ipfs?status.svg)](https://godoc.org/github.com/cryptix/git-remote-ipfs) for usage.

A 'native' git protocol helper to push and pull git repos from ipfs.

`go get -u github.com/cryptix/git-remote-ipfs`


```
 $ git clone ipfs://$hash/repo.git
 $ cd repo && make $stuff
 $ git commit -a -m 'done!'
 $ git push origin
 => clone-able as ipfs://$newHash/repo.git
```
## other tools

* [git-ipfs-rehost](https://github.com/whyrusleeping/git-ipfs-rehost) helps to push an existing repo to ipfs and make it cloneable over http://
* [gitremote-helpers docu](https://git-scm.com/docs/gitremote-helpers) canonical git documentation about remote helpers
