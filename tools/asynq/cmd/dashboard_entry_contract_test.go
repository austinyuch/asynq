package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestDashboardTTYEntryChild(t *testing.T) {
	if os.Getenv("ASYNQ_DASH_TTY_CHILD") == "" {
		return
	}
	defer fmt.Println("DASH_TTY_DEFER_RETURNED")
	rootCmd.SetArgs([]string{"--config", os.Getenv("ASYNQ_DASH_TTY_CHILD_CONFIG"), "--uri", os.Getenv("ASYNQ_DASH_TTY_TEST_REDIS_ADDR"), "--db", "12", "dash", "--refresh=1s"})
	Execute()
	fmt.Println("DASH_TTY_EXECUTE_RETURNED")
	ackFD, err := strconv.Atoi(os.Getenv("ASYNQ_DASH_TTY_ACK_FD"))
	if err != nil {
		t.Fatal(err)
	}
	ackPipe := os.NewFile(uintptr(ackFD), "owned-parent-ack")
	defer ackPipe.Close()
	var ack string
	if _, err := fmt.Fscanln(ackPipe, &ack); err != nil || ack != "DASH_TTY_PARENT_ACK" {
		t.Fatalf("parent cleanup observation ACK: %q %v", ack, err)
	}
}

type dashboardTTYObservation struct {
	Exit               int      `json:"exit"`
	Title              bool     `json:"title"`
	Help               bool     `json:"help"`
	Returned           bool     `json:"returned"`
	Deferred           bool     `json:"deferred"`
	TerminalRestored   bool     `json:"terminal_restored"`
	BeforeSize         int      `json:"before_size"`
	AfterSize          int      `json:"after_size"`
	NewClientIDs       []string `json:"new_client_ids"`
	RemainingClientIDs []string `json:"remaining_client_ids"`
	Transcript         string   `json:"transcript"`
}

func TestDashboardTTYEntryContracts(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("owned Linux PTY contract")
	}
	addr := os.Getenv("ASYNQ_DASH_TTY_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("opt-in task-owned empty DB12 required")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "owned-pty-driver.py")
	if err := os.WriteFile(script, []byte(dashboardTTYDriver), 0600); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(config, []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"normal", "unknown-term"} {
		t.Run(mode, func(t *testing.T) {
			evidence := os.Getenv("ASYNQ_DASH_TTY_EVIDENCE_DIR")
			if evidence == "" {
				evidence = dir
			}
			if err := os.MkdirAll(evidence, 0700); err != nil {
				t.Fatal(err)
			}
			profile := ""
			if testing.CoverMode() != "" && os.Getenv("ASYNQ_DASH_TTY_COVERAGE_DIR") != "" {
				profile = filepath.Join(os.Getenv("ASYNQ_DASH_TTY_COVERAGE_DIR"), mode+".cover")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			t.Cleanup(cancel)
			cmd := exec.CommandContext(ctx, "python3", script, exe, config, addr, mode, filepath.Join(evidence, mode+".transcript.bin"), profile)
			cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
			cmd.WaitDelay = 4 * time.Second
			cmd.Env = os.Environ() // HOME is inherited; no terminal/session factories.
			output, err := cmd.CombinedOutput()
			if ctx.Err() != nil {
				t.Fatalf("owned PTY parent deadline: %s", output)
			}
			if err != nil {
				t.Fatalf("PTY driver failed: %v %s", err, output)
			}
			var o dashboardTTYObservation
			if err := json.Unmarshal(output, &o); err != nil {
				t.Fatalf("driver JSON: %v %s", err, output)
			}
			if err := os.WriteFile(filepath.Join(evidence, mode+".observation.json"), output, 0600); err != nil {
				t.Fatal(err)
			}
			if o.BeforeSize != 0 || o.AfterSize != 0 {
				t.Fatalf("read-only DB12 changed: %+v", o)
			}
			if !o.TerminalRestored {
				t.Fatalf("terminal attributes not restored: %+v", o)
			}
			if len(o.RemainingClientIDs) != 0 {
				t.Fatalf("owned Inspector clients remain: %+v", o)
			}
			if mode == "normal" {
				if o.Exit != 0 || !o.Title || !o.Help || !o.Returned || !o.Deferred || len(o.NewClientIDs) == 0 {
					t.Fatalf("real public dashboard entry failed: %+v", o)
				}
			} else {
				if o.Exit != 1 || o.Returned || o.Deferred || len(o.NewClientIDs) != 0 {
					t.Fatalf("unknown TERM must exit before Inspector admission: %+v", o)
				}
				transcript, err := os.ReadFile(o.Transcript)
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(transcript), "failed to create a screen:") || !strings.Contains(string(transcript), "asynq-contract-unknown-terminal") {
					t.Fatalf("unknown TERM did not reach actual screen creation failure: %s", transcript)
				}
			}
			if strings.Contains(string(output), "DASH_TTY_TIMEOUT") {
				t.Fatal("timeout is diagnostic, not contract PASS")
			}
		})
	}
}

const dashboardTTYDriver = `
import os,sys,pty,termios,fcntl,struct,subprocess,select,time,json,socket,re,hashlib,signal
def interrupted(signum,frame):raise RuntimeError("owned PTY driver canceled; finally kills and joins child")
signal.signal(signal.SIGTERM,interrupted)
exe,config,addr,mode,transcript,profile=sys.argv[1:]
host,port=addr.rsplit(':',1)
sock=socket.create_connection((host,int(port)),timeout=2);stream=sock.makefile('rb')
def redis(*args):
 data=b'*'+str(len(args)).encode()+b'\r\n'+b''.join(b'$'+str(len(str(a).encode())).encode()+b'\r\n'+str(a).encode()+b'\r\n' for a in args)
 sock.sendall(data);line=stream.readline();tag=line[:1];body=line[1:-2]
 if tag==b':':return int(body)
 if tag==b'+':return body.decode()
 if tag==b'$':n=int(body);v=stream.read(n);assert stream.read(2)==b'\r\n';return v.decode()
 raise RuntimeError('unexpected RESP '+repr(line))
redis('SELECT',12);before_size=redis('DBSIZE');assert before_size==0,'nonempty DB12; no writes performed'
def clients():return {d['id']:d for d in (dict(part.split('=',1) for part in line.split()) for line in redis('CLIENT','LIST').splitlines()) if d.get('db')=='12'}
baseline=clients();master,slave=pty.openpty();ack_r,ack_w=os.pipe();fcntl.ioctl(slave,termios.TIOCSWINSZ,struct.pack('HHHH',48,120,0,0));original=termios.tcgetattr(slave)
env=dict(os.environ,TERM='xterm-256color' if mode=='normal' else 'asynq-contract-unknown-terminal',ASYNQ_DASH_TTY_CHILD='1',ASYNQ_DASH_TTY_CHILD_CONFIG=config,ASYNQ_DASH_TTY_ACK_FD=str(ack_r))
args=[exe,'-test.run=^TestDashboardTTYEntryChild$','-test.count=1']
if profile:args+=['-test.coverprofile='+profile]
def controlling():os.setsid();fcntl.ioctl(slave,termios.TIOCSCTTY,0)
p=None;raw=bytearray();owned=set();title=help_seen=False;phase=0;deadline=time.monotonic()+12;live_remaining=[];live_restored=None;acked=False;help_offset=None;return_offset=None
try:
 p=subprocess.Popen(args,stdin=slave,stdout=slave,stderr=slave,env=env,preexec_fn=controlling,close_fds=True,pass_fds=(ack_r,))
 while time.monotonic()<deadline:
  if select.select([master],[],[],0.05)[0]:
   try:chunk=os.read(master,65536)
   except OSError:chunk=b''
   raw.extend(chunk)
  plain=re.sub(rb'\x1b\[[0-?]*[ -/]*[@-~]',b'',bytes(raw));title=title or b'=== Queues ===' in plain
  if help_offset is not None:
   help_fragment=re.sub(rb'\x1b\[[0-?]*[ -/]*[@-~]',b'',bytes(raw[help_offset:]))
   help_seen=help_seen or b'=== Help ===' in help_fragment
  current=clients();new=set(current)-set(baseline);owned.update(new)
  healthy=any(current[i].get('cmd')=='smembers' for i in new)
  if mode=='normal':
   if phase==0 and title and healthy:help_offset=len(raw);os.write(master,b'?');phase=1
   elif phase==1 and help_seen:return_offset=len(raw);os.write(master,b'q');phase=2
   elif phase==2 and b'=== Queues ===' in re.sub(rb'\x1b\[[0-?]*[ -/]*[@-~]',b'',bytes(raw[return_offset:])):os.write(master,b'q');phase=3
  if not acked and b'DASH_TTY_EXECUTE_RETURNED' in raw:
   live_remaining=sorted(owned & set(current));live_restored=termios.tcgetattr(slave)==original
   os.write(ack_w,b'DASH_TTY_PARENT_ACK\n');acked=True
  if p.poll() is not None:break
 else:raise RuntimeError('DASH_TTY_TIMEOUT; owned child killed and joined')
 p.wait(timeout=2)
 while select.select([master],[],[],0.05)[0]:
  try:chunk=os.read(master,65536)
  except OSError:break
  if not chunk:break
  raw.extend(chunk)
 after=clients();remaining=sorted(set(live_remaining) | (owned & set(after)));restored=termios.tcgetattr(slave)==original and (live_restored is not False)
 after_size=redis('DBSIZE');open(transcript,'wb').write(raw)
 print(json.dumps(dict(exit=p.returncode,title=title,help=help_seen,returned=b'DASH_TTY_EXECUTE_RETURNED' in raw,deferred=b'DASH_TTY_DEFER_RETURNED' in raw,terminal_restored=restored,before_size=before_size,after_size=after_size,new_client_ids=sorted(owned),remaining_client_ids=remaining,transcript=transcript,binary_sha256=hashlib.sha256(open(exe,"rb").read()).hexdigest(),driver_sha256=hashlib.sha256(open(__file__,"rb").read()).hexdigest(),live_owner_observed=acked)))
finally:
 if p is not None and p.poll() is None:p.kill();p.wait(timeout=2)
 open(transcript,'wb').write(raw)
 os.close(ack_r);os.close(ack_w);os.close(master);os.close(slave);stream.close();sock.close()
`
