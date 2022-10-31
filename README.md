## Requirements:
1. [Buf](https://buf.build/)
2. [protoc](https://grpc.io/docs/protoc-installation/)
3. protoc go plugins: 
```sh
$ go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
$ go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

## Running
The easiest way to test everything is to run integration tests suite. `make integration` will generate GRPC stubs, build server and client binaries, and run the tests. Internally, the suite runs the job server, which requires root privileges. If nopaswd options is not set in `sudpers` for the current user, sudo password will be asked
```sh
ilyaz@host --- _teleport/jobs ‹server› » make integration | head    
go build -o build/jserver ./cmd/server/main.go
go build -o build/jctrl ./cmd/client/main.go
go test -v ./integration/integration_test.go --server ../build/jserver --client ../build/jctrl --uid 1000 --gid 1000
&{WorkRoot: Superusers:{FullAccess:[george] ReadAccess:[george ringo]} TLS:{CAPath:assets/cert/ca/rootCA.pem CertPath:assets/cert/server/server-cert.pem KeyPath:assets/cert/server/server-key.pem ReloadSec:30} IDs:{UID:1000 GID:1000} Address:localhost:7799}
<nil> INF added full superusers: [george]
<nil> INF added read superusers: [george ringo]
<nil> INF listening on localhost:7799
=== RUN   TestPs
<nil> INF Calling /jobs.v1.JobService/Start client=john
<nil> INF Start proc for: "/proc/self/exe" [/proc/self/exe --mode=shim --cmd=ps --cgroup=/sys/fs/cgroup/job-cde92hsran1741nbmun0/inner --uid=1000 --gid=1000 -- aux] id=cde92hsran1741nbmun0
make: *** [Makefile:44: integration] Broken pipe
...
```
## Building
run `make client server` to build binaries:
- `build/jserver` - server
- `build/jctrl` - client

## Client
### Configuration
to make any request to server the following parameters required:
- `—server’ - job server endpoint
- ‘—capath’ - path to the CA certificate to verify server TLS certificate
- ‘—cert’ - path to the client TLS certificate
- ‘—key’ - path to the client TLS key
for example:
```sh
ilyaz@skeleton --- integration/assets ‹server* ?› » jctrl --capath cert/ca/rootCA.pem --cert cert/client/john-cert.pem --key cert/client/john-key.pem --server "[::1]:7799" run ls         

cdeqaa4ran13fq8tqu60
```
To make things a bit simpler, `config` command exists. It puts all the passed args to a file

```sh
ilyaz@skeleton --- integration/assets ‹server* ?› » jctrl --capath cert/ca/rootCA.pem --cert cert/client/john-cert.pem --key cert/client/john-key.pem --server "[::1]:7799"  config  --config my.yaml
ilyaz@skeleton --- integration/assets ‹server* ?› » cat my.yaml
server: '[::1]:7799'
capath: cert/ca/rootCA.pem
cert: cert/client/john-cert.pem
key: cert/client/john-key.pem
ilyaz@skeleton --- integration/assets ‹server* ?› » 
ilyaz@skeleton --- integration/assets ‹server* ?› » jctrl --config my.yaml run id                   
cdeqdo4ran13fq8tqu6g
ilyaz@skeleton --- integration/assets ‹server* ?› » jctrl --config my.yaml logs cdeqdo4ran13fq8tqu6g
uid=1000(ilyaz) gid=1000(ilyaz) groups=1000(ilyaz)
```

If `—config` is not passed, `jctrl` will try to find and use the file `jctrl.yaml` in the current directory and in the user’s home directory.

It’s possible to override values from the config file by command line args

```sh
ilyaz@skeleton --- integration/assets ‹server* ?› » jctrl --config my.yaml --cert cert/client/paul-cert.pem --key cert/client/paul-key.pem  run id
cdeqes4ran13fq8tqu70
ilyaz@skeleton --- integration/assets ‹server* ?› »
```

here we use pre-configured `CA` and `server`, but with other client identity

Below we're using the following config for simpicity:
```sh
ilyaz@skeleton --- integration/assets ‹server* ?› »  jctrl --capath cert/ca/rootCA.pem --cert cert/client/john-cert.pem --key cert/client/john-key.pem --server "[::1]:7799"  config             
ilyaz@skeleton --- integration/assets ‹server* ?› » cat jctrl.yaml 
server: '[::1]:7799'
capath: cert/ca/rootCA.pem
cert: cert/client/john-cert.pem
key: cert/client/john-key.pem
```

### Running a job


```sh
ilyaz@skeleton --- integration/assets ‹server* ?› » jctrl run -- sh -c "ip link; echo; ps aux"                                                                                                1 ↵
cdeqhksran13fq8tqu7g
ilyaz@skeleton --- integration/assets ‹server* ?› » jctrl logs cdeqhksran13fq8tqu7g
1: lo: <LOOPBACK> mtu 65536 qdisc noop state DOWN mode DEFAULT group default qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00

    USER         PID %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND
    ilyaz          1  0.0  0.0 1240084 10616 pts/10  Sl+  15:27   0:00 /proc/self/exe --mode=shim --cmd=sh --cgroup=/sys/fs/cgroup/job-cdeqhksran13fq8tqu7g/inner --uid=1000 --gid=1000 -- -c ip link; echo; ps aux
    ilyaz          8  0.0  0.0   2888  1004 pts/10   S+   15:27   0:00 sh -c ip link; echo; ps aux
    ilyaz         11  0.0  0.0  21324  1568 pts/10   R+   15:27   0:00 ps aux
  ```



### Stopping a job
`stop` commands stops the jobs. If current user (the one we pass in cert) is regular, it’s possible to stop only jobs started with the same user id. Another option is that the current user is super-user with full-access privileges, in this case they can stop any active job.

```sh
ilyaz@skeleton --- integration/assets ‹server* ?› » jctrl --config my.yaml --cert cert/client/paul-cert.pem --key cert/client/paul-key.pem stop cdeqijcran13fq8tqu8g                        130 ↵
failed to stop the job: no such job
ilyaz@skeleton --- integration/assets ‹server* ?› »                                                                                                                                           1 ↵
ilyaz@skeleton --- integration/assets ‹server* ?› » jctrl --config my.yaml  stop cdeqijcran13fq8tqu8g                                                                                         1 ↵
ilyaz@skeleton --- integration/assets ‹server* ?› » jctrl inspect cdeqijcran13fq8tqu8g                                                                                                        1 ↵
Job:            cdeqijcran13fq8tqu8g
Command:        sh -c while true; do date; sleep 1; done
Status:         STATUS_STOPPED
ExitCode:       255
```
Please note, when trying to get unauthorized access to a job, `no such job` error returned instead of `no access` for security reasons.

```sh
ilyaz@skeleton --- integration/assets ‹server* ?› » jctrl run -- sh -c 'while true; do date; sleep 1; done'                                                                                 130 ↵
cdequdsran13fq8tqua0
ilyaz@skeleton --- integration/assets ‹server* ?› » jctrl --cert cert/client/george-cert.pem --key cert/client/george-key.pem stop cdequdsran13fq8tqua0 # user ‘george’ is super-user
ilyaz@skeleton --- integration/assets ‹server* ?› » 

```
by default stop operation is graceful, to make forced stop please use `-f` option


### Getting output 
command `logs` get the combined stdout and stderr output of a job
‘Follow’ mode supported, enabled with `-f` option. Client will wait for more output until the job is ended

```sh
ilyaz@skeleton --- integration/assets ‹server* ?› » jctrl run -- sh -c 'while true; do date; sleep 5; done'
cder9s4ran13fq8tqub0
ilyaz@skeleton --- integration/assets ‹server* ?› » jctrl logs cder9s4ran13fq8tqub0                        
Sat Oct 29 04:19:12 PM PDT 2022
Sat Oct 29 04:19:17 PM PDT 2022
Sat Oct 29 04:19:22 PM PDT 2022
ilyaz@skeleton --- integration/assets ‹server* ?› » 
ilyaz@skeleton --- integration/assets ‹server* ?› » 
ilyaz@skeleton --- integration/assets ‹server* ?› » jctrl logs -f cder9s4ran13fq8tqub0
Sat Oct 29 04:19:12 PM PDT 2022
Sat Oct 29 04:19:17 PM PDT 2022
Sat Oct 29 04:19:22 PM PDT 2022
Sat Oct 29 04:19:27 PM PDT 2022
Sat Oct 29 04:19:32 PM PDT 2022

Sat Oct 29 04:19:37 PM PDT 2022
^C
ilyaz@skeleton --- integration/assets ‹server* ?› »                                                                                                                                         130 ↵

```

### Inspecting a job
Please use `inspect` command. A job can be inspected by starting user, or by super-user with full-read, or full access

```sh
ilyaz@skeleton --- integration/assets ‹server* ?› » jctrl run -- sh -c "exit 77"      
cdeqk3cran13fq8tqu9g
ilyaz@skeleton --- integration/assets ‹server* ?› » jctrl inspect cdeqk3cran13fq8tqu9g
Job:            cdeqk3cran13fq8tqu9g
Command:        sh -c exit 77
Status:         STATUS_ENDED
ExitCode:       77
```

### Cleaning up
`rm` command  removes stopped or ended job. Write permissions required

  ```sh
    ilyaz@skeleton --- integration/assets ‹server* ?› » jctrl rm cdeqk3cran13fq8tqu9g
    ilyaz@skeleton --- integration/assets ‹server* ?› » jctrl inspect cdeqk3cran13fq8tqu9g
    failed to inspect: no such job
    
  ```

## Server
A config file required to run the server:

```sh
ilyaz@skeleton --- _teleport/jobs ‹server* ?› » cat config/test-server.yaml 

tls:
# CA cert to verify clients
  capath: integration/assets/cert/ca/rootCA.pem
  # TLS server certificate
    cert: integration/assets/cert/server/server-cert.pem
# TLS  server certificate key
  key: integration/assets/cert/server/server-key.pem
  # server endpoint
  address: "[::1]:7799"
  ids:
  # user ID to use for jobs. can be a username
    uid: 1000
    # group ID to use for jobs. can be a groupname
    gid: 1000

  superusers:
  # a list of users who have full access to all jobs
    full:
    - george
    # a list of users who have read (logs + inspect) access to all jobs	
    read:
    - george
    - ringo
  ```

  ```sh
  ilyaz@skeleton --- _teleport/jobs ‹server* ?› » sudo build/jserver --config config/test-server.yaml
  <nil> INF added full superusers: [george]
  <nil> INF added read superusers: [george ringo]
  <nil> INF reload cert from (integration/assets/cert/server/server-cert.pem/integration/assets/cert/server/server-key.pem) every 30s
  <nil> INF listening on [::1]:7799
  ````


  ### Server certificate rollover
  The server tries to reload TLS certificate every 30 seconds from the same cert/key locations.
  If new filed are not valid, a warning is logged and the last successful configuration is used


  ### Logs
  Server logs all API request, including client id, timing, and any recovered panics.

  ```sh
  2022-10-30T14:08:42-07:00 INF Calling /jobs.v1.JobService/Start client=john
  2022-10-30T14:08:42-07:00 INF Start proc for: "/proc/self/exe" [/proc/self/exe --mode=shim --cmd=sh --cgroup=/sys/fs/cgroup/job-cdfefmkran1467ku472g/inner --uid=1000 --gid=1000 -- -c while true; do date; sleep 2; done] id=cdfefmkran1467ku472g
  2022-10-30T14:08:42-07:00 INF Method /jobs.v1.JobService/Start took 35.32274ms client=john
  ```

  ### Stopping the server
  on Ctrl-C, the server tries to shutdown gracefully, it starts graceful shutdown on all jobs, then does full cleanup and then exit.
