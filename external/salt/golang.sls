golang-{{ pillar.golang.version }}:
  archive.extracted:
    - name: /usr/local/src/go-{{ pillar.golang.version }}
    - source: https://storage.googleapis.com/golang/go{{ pillar.golang.version }}.linux-amd64.tar.gz
    - source_hash: {{ pillar.golang.hash }}
    - archive_format: tar

/usr/local/bin/go:
  file.symlink:
    - target: /usr/local/src/go-{{ pillar.golang.version }}/go/bin/go

/usr/local/bin/gofmt:
  file.symlink:
    - target: /usr/local/src/go-{{ pillar.golang.version }}/go/bin/gofmt

/usr/local/bin/godoc:
  file.symlink:
    - target: /usr/local/src/go-{{ pillar.golang.version }}/go/bin/godoc

/etc/profile.d/golang.sh:
  file.managed:
    - contents: |
        export GOPATH=$HOME/go
        test -d $GOPATH || mkdir $GOPATH
    - mode: 755
