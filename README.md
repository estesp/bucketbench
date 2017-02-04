# bucketbench
Bucketbench is a simple framework for running defined sequences
of lifecycle container operations against three different container
engines today: the full Docker engine, OCI's runc, and containerd.

Given a **bucket** is a physical type of container, the name is my attempt to
get away from calling it "dockerbench," given it runs against other
container engines as well. All attempts to come up with a more interesting
name failed before initial release. Suggestions welcome!

## Background
This project came about via some performance comparison work happening
in the [OpenWhisk](https://openwhisk.org) serverless project. Developers
in that project had a python script for doing similar comparisons, but
agreed we should extend it to a more general framework which could be
easily be extended for other lifecycle operation sequences, as the python
script was hardcoded to a specific set of operations.

## Usage
The project as it stands today only has a single benchmark implemented.
This "Basic" benchmark performs a **start/run** operation followed by
**stop** and **remove** operations. Because benchmarks are abstracted from
driver implementation, copying `basic.go` and creating a different set of
operations would be very simple. All three drivers implement a small set
of lifecycle operations (defined as an interface in `driver/driver.go`), and
any benchmark can mix/match any of those operations as a benchmark run.

Specific command usage for the `bucketbench` program is as follows:
```
Providing the number of threads selected for each possible engine, this
command will run those number of threads with the pre-defined lifecycle commands
and then report the results to the terminal.

Usage:
  bucketbench run [flags]

Flags:
  -b, --bundle string          Path of test runc image bundle (default ".")
  -c, --containerd int         Number of threads to execute against containerd
      --ctr-binary string      Name/path of containerd client (ctr) binary (default "ctr")
  -d, --docker int             Number of threads to execute against Docker
      --docker-binary string   Name/path of Docker binary (default "docker")
  -i, --image string           Name of test Docker image (default "busybox")
  -r, --runc int               Number of threads to execute against runc
      --runc-binary string     Name/path of runc binary (default "runc")
  -t, --trace                  Enable per-container tracing during benchmark runs

Global Flags:
      --log-level string   set the logging level (info,warn,err,debug) (default "warn")
```

A common invocation might look like:
```
$ sudo ./bucketbench --log-level=debug run -b ~/containers/btest -i btest -d 3 -r 3
```

This invocation will use an OCI bundle I've put in my home directory under `./containers/btest` and
a Docker image named `btest`, running a set of fixed container operations (the "Basic" benchmark)
as a single thread, then two, and finally three concurrent threads, ending with output similar to
the following to show the overall rate (iterations of the operations/second) for each of the thread counts:
```
             Iter/Thd     1 thrd  2 thrds  3 thrds  4 thrds  5 thrds  6 thrds  7 thrds  8 thrds  9 thrds 10 thrds
Limit            1000    1171.24  1957.17  2101.13  2067.83  1827.92  1637.32  1257.57  1582.36  1306.08  1699.56
DockerBasic        15       1.40     2.21     2.81
RuncBasic          50       8.38    15.85    23.00
```

The "Limit" test is fixed to run up through ten threads, but as we specified the `runc` and `docker`
drivers were only executed up through three threads. The iterations/thread number is how many times
each thread executes the specified list of lifecycle operations. In the case of the "Basic" benchmark,
this is **create/start**, **stop**, followed by **rm**. You can see that we are getting reasonable
scaling as we increase threads/workers in the `runc` case. More details on these findings will be available
in a blog post as well as a set of conference talks in 2017.

This initial implementation is very simple until further expansion of the client interface
allows the user to specify which benchmarks to run. Until that TODO item is handled, this first pass implementation
runs the "Limit" benchmark--which is useful only to show the rate at which the local host
can `exec()` processes, used as a basic "thumb in the air" comparison between systems--followed
by the "Basic" benchmark, executed against any engines specified with one or more threads using
the client arguments.

To run the test against `runc` or `containerd` you must `sudo bucketbench` because of the
requirements that those tools have for root access. This tool does not manage the two daemon-based
engines (containerd and dockerd), and will fail if they are not up and running at benchmark runtime.

The tool will start a significant number of containers against these daemons, but attempts to
fully cleanup after running each iteration.

## Development Notes

The `bucketbench` tool is most likely only valuable on amd64/linux, as `containerd` and `runc` are
delivered today as binaries for those platforms. It will most likely build for other platforms, and
if run against a tool like Docker for Mac, would probably work against the Docker engine, but not
against `containerd` or `runc`.

All the necessary dependencies are vendored into the `bucketbench` tree, so building should be as
easy as `go build -o bucketbench .`, and `go install github.com/estesp/bucketbench` should work as
well.

## TODOs

 - Need to expand client UX for specifying benchmarks to run.
 - An easier way to define benchmarks via an input format (JSON/YAML?) rather than code?
 - Better design of statistics being gathered (and currently unused) for each operation's metrics;
   currently hardcoded for "basic" benchmark list of operations and would not be useful anywhere else.
 - Revisit setup/cleanup hardcoded actions that may or may not be valid in the general case.
