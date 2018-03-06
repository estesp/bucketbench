.PHONY: all binary static clean install

PREFIX ?= ${DESTDIR}/usr
INSTALLDIR=${PREFIX}/bin
MANINSTALLDIR=${PREFIX}/share/man

GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
COMMIT_NO := $(shell git rev-parse HEAD 2> /dev/null || true)
COMMIT := $(if $(shell git status --porcelain --untracked-files=no),"${COMMIT_NO}-dirty","${COMMIT_NO}")

all: binary

# Target to build a dynamically linked binary
binary:
	go build -ldflags "-X github.com/estesp/bucketbench/cmd.gitCommit=${COMMIT}" -o bucketbench github.com/estesp/bucketbench

# Target to build a statically linked binary
static:
	GO_EXTLINK_ENABLED=0 CGO_ENABLED=0 go build \
	   -ldflags "-w -extldflags -static -X github.com/estesp/bucketbench/cmd.gitCommit=${COMMIT}" \
	   -tags netgo -installsuffix netgo \
	   -o bucketbench github.com/estesp/bucketbench

clean:
	rm -f bucketbench

install:
	install -d -m 0755 ${INSTALLDIR}
	install -m 755 bucketbench ${INSTALLDIR}
