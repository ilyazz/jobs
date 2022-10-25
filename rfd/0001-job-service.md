---
authors: Ilya Zuyev (ilya.zuyev@gmail.com)
state: draft
---

# RFD 0001 - Job service

## What
Implement a prototype job worker service that provides an API to run arbitrary Linux processes.

## Why
The service implements requirements described in https://github.com/gravitational/careers/blob/main/challenges/systems/challenge-1.md#level-5 It’s intended to be used for demo purposes only, no real customers expected. The implementation time is limited to 2 weeks, so the scope should be planned accordingly.
Areas of focus:
 - client/server networking
 - security
 - code quality

## Details

The following components must be delivered:

1. Library written in Go, providing abilities to:
   - start a new job,
   - stop a job managed by the service instance
   - get the current status of a job started by the service instance. This should work for all kinds of jobs, including active, ended and failed to start
   - get a stream of combined stdout/stderr output of a job started by the service instance. This should work for both active and stopped jobs 
2. Server binary, providing external GRPC API over mTLS with the strict security rules. The server API must provide the same operations as the library. Supported systems: Linux/any arch.
3. A CLI client tool to interact with the server. The tool must provide the same set of operations as the server.

### Not Doing
The following topics are consciously ignored:

- Protocol versioning
- Client/Server versioning
- Advanced configuration management
- Observability (excluding simple logging)
- Deployment scripts, server management tools
- Certificates revocation
- CA rollovers
- Audit
- Version upgrades
- Advanced UI features like i19n and accessibility


## Implementation

### Client
The following CLI commands are proposed

#### Start
Starts a new job
```shell
--- ~ » #jctl [COMMON_OPTIONS] start [OPTIONS] [COMMAND] [ARGS]
--- ~ » jctl --server localhost:1966 --cert client.crt --key client-key.pem --ca ca.crt start --cpus 1.5 --mem 1g --io 1g ps -a  
```
This command returns the job ID if the job was actually started.

Options are:
- `-cpus` the CPU resource usage limit. Can be greater than zero and fractional. e.g. `--cpus=3.5`. Defines `cpu.max` cgroup file. Zero means no limit.
- `-mem` memory limit. Max process memory in bytes. Defines `memory.max` cgroup file. Zero means no limit 
- `-io` io rate limit per block device. Defines `wpbs` and `rbps` values of `io.max` cgroup file. Zero means no limit.

#### Stop
Stops the job by ID
```shell
--- ~ » #jctl [COMMON_OPTIONS] stop [OPTIONS] [JID]
--- ~ » jctl stop -i 157acdc  
```
(Common options omitted for clarity)
Options are:
- `-i` to use the immediate stop strategy
- `-g` to use the graceful stop strategy

#### Logs
Get a job output stream
```shell
--- ~ » #jctl [COMMON_OPTIONS] logs [OPTIONS] [JID]
--- ~ » jctl logs -f 157acdc  
```
(Common options omitted for clarity)

Options are:
- `-f` follow output

#### Inspect
Get a job status
```shell
--- ~ » #jctl [COMMON_OPTIONS] inspect [JID]
--- ~ » jctl inspect 157acdc  
```

#### Rm
Cleanup job artifacts
```shell
--- ~ » #jctl [COMMON_OPTIONS] rm [JID]
--- ~ » jctl rm 157acdc  
```
(Common options omitted for clarity)

All commands require common options:
- `-server` - server URL
- `-cert` - path to a client certificate
- `-key` - path to a client cetificate key
- `-ca` - path to a CA certificate

Additionally, all commands support:
- `-h` options to get usage info
- `-v` option to print logs.

---
## Library
The following diagram shows the usage of the library in the server architecture
```
             +----------------------------------------------------------+                                    
             |               Service Process                            |                                    
             |  +---------+  +------------------------------------------+                                    
             |  |         |  |                                          |                                    
             |  |         |  |             +------------------------+   |   +--------------------+           
  External   |  |         |  |             |                        +---+-->|      Job           |           
    API      |  |         |  |             | Job Handler            |   |   |      Process       |           
     o-------+--+-        |  |   S         +------------------------+   |   +--------------------+           
             |  |    G    |  |   u                                      |                                    
             |  |    R    |  |   p         +------------------------+   |   +--------------------+           
             |  |    P    |  |   e         |                        +---+-->|      Job           |           
             |  |    C    |->|   r         | Job Handler            |   |   |      Process       |           
             |  |         |  |   v         +------------------------+   |   +--------------------+           
             |  |         |  |   i                                      |                                    
             |  |         |  |   s                                      |                                    
             |  |         |  |   o                                      |                                    
             |  |         |  |   r                                      |                                    
             |  |         |  |             +------------------------+   |   +--------------------+           
             |  |         |  |             |                        +---+-->|      Job           |           
             |  |         |  |             | Job Handler            |   |   |      Process       |           
             |  |         |  |             +------------------------+   |   +--------------------+           
             |  |         |  |                                          |                                    
             |  +---------+  +------------------------------------------+                                    
             |                                                          |                                    
             +----------------------------------------------------------+                                    
```

Job controller is implemented as `Supervisor` object. It owns a set of `Job Handler` objects, each contains job data, like IDs and states.

`Job Handler` implementation is a simple FSM with the following state diagram:

```
                                     +-------------+
                               +-+   |  Failed     |
                               |x+-->|  to         |
                               +++   |  start      |
                                |    +-------------+
                                v
                         +----------------+
                         |                |
                         |  Running       +-------------+
                         |                |             |
                         +------+---------+             |
                                |                       |
                                v                       v
                         +-----------------+     +--------------+
                         |                 |     |              |
                         |  Ending         +---->|  Ended       |
                         |                 |     |              |
                         +-----------------+     +--------------+
```

The following API is proposed for the library API:
```go
package job

// ID is the type to represent job ID
type ID string

// Status contains details of a current state of job,
// including exit code (if any)
type Status struct {
	//
}

// StartOptions defines parameters of Start operation
type StartOptions struct {
	// ...
}

// StopOptions defines parameters of Stop operation
type StopOptions struct {
	// ...
}

// OutputOptions defines parameters of Output operation
type OutputOptions struct {
	// ...
}

// Supervisor is the main Job Control interface
type Supervisor interface {
	// StartJob starts a new job using options o. Generates a new random job ID
	StartJob(ctx context.Context, o *StartOptions) (ID, error)
	// StopJob stops the job id using options o
	StopJob(ctx context.Context, id ID, o *StopOptions) (Status, error)
	// RemoveJob does cleanup for a job, removing its handler, and disk artifacts
	RemoveJob(ctx context.Context, id ID) error
	// InspectJob returns details about the job
	InspectJob(ctx context.Context, id ID) (Status, error)
	// JobOutput returns the combined stdout/stderr output of a job, along the job status
	JobOutput(ctx context.Context, id ID, o *OutputOptions) (Status, io.ReaderCloser, error)

	// Stop stops the supervisor and all its jobs
	Stop() error
}

// SupervisorOptions defines parameters of Supervisor service
type SupervisorOptions struct {
	// ...
}

// New creates a new supervisor instance
func New(o *SupervisorOptions) (Supervisor, error) {
	//
}
```
All Supervisor methods are thread-safe.

---
The preferred way to work with OS objects is to use Go standard library objects from packages like `os` or `signal`, instead of direct system calls from the package `syscall`.

#### Start
Before we run the target command, we have to configure namespaces and cgroup limits. That's why we cannot just create and run a new `exec.Command`instance. For example, we have to have a PID of the new process to configure cgroups after the process is started but before the target binary begins execution.
It's proposed to use the [fork/exec](https://en.wikipedia.org/wiki/Fork%E2%80%93exec) technique.
Steps are as follows:
1. Generate a new unique job ID `<job_id>`
2. Make a directory structure for the new job - `/var/run/jobs/<job_id>/{wd, out}`, where `wd` is an empty working directory, and `out` is the place we'll store the command output.
3. Create a named Linux pipe. Will be explained later. 
4. Start a new process. Use `exec.Command` interface with `/proc/self/exe` as a name, passing `MODE=shim` and command parameters in environment variables .Another approach would be to build and execute a special shim binary, but for the simplicity let's reuse the server binary. Redirect stdout and stderr to a binary file `/var/run/jobs/<job_id>/out/out`. Pass the pipe writer as an additional FD.
5. In the new process:
   1. Detect it is started in `shim` mode by checking environment variables.
   2. Do sanity check, to make sure the process started correctly. Validate, that the current process' command is `/proc/self/exe`
   3. The process is started in a new network namespace, thus network configuration is minimal. The process has only the loopback device visible, routing rules are empty. Do the minimal configuration - enable the `lo` device. 
   4. Configure the filesystems. The process inherits the mounts from its parent. To make tools like `ps` work correctly, unmount the `/proc` FS and mount it again.
   5. Configure the cgroups parameters. Create a new controller, set limits for the number of CPUs, available memory and disk IO max rate.
   6. We have to lower the privileges. Take the target UID and GID from passed configuration, call `setuid` and `setgid`.
   7. It's possible that any operation on steps i-vi fails. In this case we have to report the error to the parent service process and exit. Use the named pipe from the step 3 for passing the error information.
   8. Close the pipe
   9. Run `syscall.exec` passing the target command
  
6. In the parent process use `cmd.Wait()` to collect the new process exit data.

-------

#### Stop
It's proposed to implement two stop strategies: immediate and graceful.

For the immediate stop the supervisor just sends **SIGKILL** to the job process. This signal cannot be ignored, and the job must stop immediately.
For the graceful stop the supervisor first sends **SIGTERM**, then waits for configured time, 30 seconds by default, and then, if the process is still active, send **SIGKILL**. The operation returns after **SIGTERM** is sent, the rest is processed asynchronously. 
The strategy is chosen based on `StopOptions` argument of `StopJob` API method.

If the job is in `STOPPING` state, meaning that **SIGTERM** has been sent, but the process is still running, it's allowed to make another StopJob API call, using immediate stop, and sending **SIGKILL** immediately.
After the job is stopped, its handler is still in memory, output is on disk and is available via API.


#### Remove
*Remove* does a cleanup, removing all artifacts created by the job.
1. Remove the `/var/run/jobs/<job_id>` directory.
2. Remove the job cgroups controller
3. Remove the network namespace
4. Release the Job handler from the supervisor memory.
-------

#### Inspect

*Inspect* operation is straightforward. The supervisor just finds the Job Handle that has all the required data, and returns the data to the client.

-------

#### Logs

*Logs* operation starts a new goroutine tha opens the job output file in read-only mode and copies its content to a channel of `[]byte`. Job output can be binary, so we work with raw bytes instead of string.
The channel is returned to the client. An option is passed, allowing to use the 'follow' mode, in which when the reader reaches the end of the output file, it waits for more data.  

-------
Job processes have the server process as a parent. If it terminates, all jobs are stopped and created artifacts, like disk files or cgroups controllers remains. It's not possible to restore job states after the supervisor starts again. 


## Server

1. The server runs with root privileges.
2. At start the server binds to the configured network port on the configured network address. Both ipv6 and ipv4 are supported. No other transports, like domain sockets, are allowed.
3. A TLS GRPC server than started. See *Security* section for TLS details
4. The paths to TLS cert and key are configurable
5. It's possible to make a cert rollover and change the files on disk when the server is running. The server checks the files every 30 seconds (hardcoded) and tries to re-read them. If new cert/key are incorrect, the error is reported to logs; the server continues to user the previous versions of cert/key.
6. The server is delivered as a simple Linux binary. It has no `daemon` mode, neither supporting scripts to run it as systemd or sys-v init service.
7. The GRPC API copies the library API. Protobuf byte stream is used to deliver jobs output.

The following GRPC interface proposed: 

```protobuf
syntax = "proto3";

package jobs;
option go_package = "github.com/ilyazz/jobs/api/proto";

import "google/protobuf/empty.proto";

// job status
enum Status {
  // Job is running
  JOB_ACTIVE = 0;
  // Graceful stop activated, but not done yet
  JOB_STOPPING = 1;
  // Job is stopped via API
  JOB_STOPPED = 2;
  // Job has completed
  JOB_ENDED = 3;
}

// StopMode describes how jobs are stopped
enum StopMode {
  // Kill the job
  STOP_IMMEDIATE = 0;
  // Notify the job about stop, wait for some time, and then kill it
  STOP_GRACEFUL = 1;
}

// options related to 'Logs' API
message LogsOptions {
  // if true, server will keep the output stream open if the all output has been sent,
  // waiting for more data to arrive
  bool follow = 1;
}

// job process limits
message Limits {
  // max memory amount in bytes. 0 means no limit
  int64  memory = 1;
  // max cpus. may be a fraction, e.g. cpus=3.14. 0.0 means no limit
  float  cpus = 2;
  // max IO write and read rates in bytes. 0 means no limit
  int64  io = 3;
}

// request to start a new job
message StartRequest {
  // job command
  string command = 1;
  // limits of the job process
  Limits limits = 2;
}

// job start response
message StartResponse {
  // unique job id
  string job_id = 1;
}

// job stop request
message StopRequest {
  // id of the job to stop
  string job_id = 1;
  // stop strategy
  StopMode mode = 2;
}

// job stop response
message StopResponse {
  // job state after the stop command
  Details details = 1;
}

// request to get job details
message InspectRequest {
  // job id to inspect
  string job_id = 1;
}

// response to inspect
message InspectResponse {
  // job state
  Details details = 1;
}

// request to cleanup a stopped job
message RemoveRequest {
  // job id to remove
  string job_id = 1;
}

// request to get job output
message LogsRequest {
  // job id to get output from
  string job_id = 1;
  // how to get output
  LogsOptions options = 2;
}

// Logs API returns a GRPC stream of LogsResponseItem
message LogsResponseItem {
  // raw bytes, a chunk of output
  bytes data = 1;
}

// job details
message Details {
  // current job state
  Status status = 1;
  // process exit code if the status is JOB_STOPPED or JOB_ENDED, 0 otherwise
  int32  exit_code = 2;
}

// JobService provides methods to control jobs on server
service JobService {
  // Start a new job
  rpc Start(StartRequest) returns(StartResponse);
  // Stop active job. Another option is to force-stop a stopping job
  rpc Stop(StopRequest) returns(StopResponse);
  // Remove inactive job. Cleanup server artifacts
  rpc Remove(RemoveRequest) returns(google.protobuf.Empty);
  // Get job details
  rpc Inspect(InspectRequest) returns(InspectResponse);
  // Get a stream of job output
  rpc Logs(LogsRequest) returns(stream LogsResponseItem);
}
```
---
The following sections describe additional implementation details grouped by topic

## UX
### Client
1. The client tool should mimic the popular similar tools like [docker-cli]])(https://docs.docker.com/engine/reference/commandline/cli/), [crictl](https://github.com/kubernetes-sigs/cri-tools/blob/master/docs/crictl.md) or [nerdclt](https://github.com/containerd/nerdctl), using the similar naming and config options.
2. The client tool must provide short help/usage message for all commands
3. The logging must be provided on demand, providing enough details to investigate possible issues.
4. Error messages must be comprehensive and tell user what to do. (in a good way)

### Server
Server UX is out of scope of this project. It is delivered as a simple binary, run as a regular Linux application. No init systems support, server management tools, advanced observability, audit, or version upgrade features provided.


## Security
### Transport
1. Use mTLS for client <-> server communications. 
   1. Disable 'insecure' option on both client and server, disallowing usage of unsigned certificates.
   2. Use [Elliptic Curve](https://www.digicert.com/faq/ecc.htm) encryption for both client and server keys.
   3. Enforce TLS 1.3. This version contains [significant security improvements](https://www.ssl.com/article/tls-1-3-is-here-to-stay/) over the version 1.2. It's not required to support GRPC clients other than the CLI tool, so we don't to support older versions of TLS.
   4. Enable certificate rotation on server. 
   5. Cypher suite configuration is skipped, as it's [not recommended](https://go.dev/blog/tls-cipher-suites) for Go's TLS 1.3.
2. Use the TLS implementation from the Go standard library. It's stable, and fully implements [RFC8466](https://www.rfc-editor.org/rfc/rfc8446.html)  
3. Use Go version 1.18 - the latest version with TLS-related [improvements](https://tip.golang.org/doc/go1.18#tls10)
### Authentication
Use the client certificate Distinguished Name (DN) as the client ID
### Authorization
The following rules apply:
   - A client may access jobs started with the same client ID
   - There are "super-users", who have full access to all jobs. The list of identities of super-users is stored in a config file on server and is read only when the server starts

## Target platforms
### Server
Linux systems with any CPU arch supported by Go compiler with kernels supporting cgroup2 and net/pid/mount namespaces
### Client
Any platform supported by Go compiler

## Testing
### Unit tests
It's not required to reach some fixed level of test coverage. Identify and cover the most important part of the code.
### Integration tests
Use a docker container to run the server and test running the client tool against it.

## Observability
Due to requirements, observability is limited.
### Logs 
1. Server provides structured logs in plain text written to server's stderr. No advanced features like log rotation, format spec, or uploading to external services. 
2. Client provides structured logs on demand, enabled with `-v` option.
### Metrics
No server metrics
### Alerts
No server alerts
