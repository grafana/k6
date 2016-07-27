golang:
  pkgrepo.managed:
    - ppa: ubuntu-lxc/lxd-stable
  pkg.latest: []

/etc/profile.d/golang.sh:
  file.managed:
    - contents: |
        export GOPATH=$HOME/go
        test -d $GOPATH || mkdir $GOPATH
    - mode: 755
