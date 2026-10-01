package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/austinyuch/asynq"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type explicitConfigObservation struct {
	BodyCalled   bool
	Addr         string
	DB           int
	ExecuteError string
}

// Mutate command/config globals only in the child. The observing command uses
// the real root initializer and connection option builder without connecting.
func TestCLIConfigContractChild(t *testing.T) {
	mode := os.Getenv("ASYNQ_CLI_CONFIG_CONTRACT_CHILD")
	if mode == "" {
		return
	}
	viper.SetEnvPrefix("ASYNQ_CLI_CONFIG_CONTRACT")
	path := os.Getenv("ASYNQ_CLI_CONFIG_CONTRACT_PATH")
	if path == "" {
		t.Fatal("explicit path required; never search HOME")
	}
	if mode == "version" {
		rootCmd.SetArgs([]string{"--config", path, "version"})
		Execute()
		os.Exit(0)
	}
	var o explicitConfigObservation
	var observations []explicitConfigObservation
	rootCmd.AddCommand(&cobra.Command{Use: "config-observe", RunE: func(_ *cobra.Command, _ []string) error {
		o.BodyCalled = true
		opt := getRedisConnOpt().(asynq.RedisClientOpt)
		o.Addr = opt.Addr
		o.DB = opt.DB
		return nil
	}})
	paths := []string{path}
	if mode == "repeat" {
		if e := json.Unmarshal([]byte(path), &paths); e != nil {
			t.Fatal(e)
		}
	}
	for _, configPath := range paths {
		o = explicitConfigObservation{}
		rootCmd.SetArgs([]string{"--config", configPath, "config-observe"})
		if _, e := rootCmd.ExecuteC(); e != nil {
			o.ExecuteError = e.Error()
		}
		observations = append(observations, o)
	}
	var result any = o
	if mode == "repeat" {
		result = observations
	}
	b, e := json.Marshal(result)
	if e != nil {
		t.Fatal(e)
	}
	fmt.Println("CONFIG_OBSERVATION:" + string(b))
	os.Exit(0)
}
func runExplicitConfigChild(t *testing.T, path, mode string) (string, int) {
	t.Helper()
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "-test.run=^TestCLIConfigContractChild$")
	cmd.Env = append(os.Environ(), "ASYNQ_CLI_CONFIG_CONTRACT_CHILD="+mode, "ASYNQ_CLI_CONFIG_CONTRACT_PATH="+path)
	b, e := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("child timeout: %v %s", ctx.Err(), b)
	}
	if e == nil {
		return string(b), 0
	}
	if x, ok := e.(*exec.ExitError); ok {
		return string(b), x.ExitCode()
	}
	t.Fatal(e)
	return "", -1
}
func TestCLIExplicitConfigFailClosedContracts(t *testing.T) {
	dir := t.TempDir()
	valid := filepath.Join(dir, "valid.yaml")
	bad := filepath.Join(dir, "malformed.yaml")
	missing := filepath.Join(dir, "missing.yaml")
	for path, body := range map[string]string{valid: "uri: config-fixture.invalid:7319\ndb: 6\n", bad: "uri: [unterminated\n"} {
		if e := os.WriteFile(path, []byte(body), 0600); e != nil {
			t.Fatal(e)
		}
	}
	for _, tc := range []struct {
		name, path string
		valid      bool
	}{{"valid", valid, true}, {"missing", missing, false}, {"malformed", bad, false}} {
		t.Run(tc.name, func(t *testing.T) {
			reader := viper.New()
			reader.SetConfigFile(tc.path)
			parseErr := reader.ReadInConfig()
			if tc.valid && parseErr != nil || !tc.valid && parseErr == nil {
				t.Fatalf("fixture validity mismatch: %v", parseErr)
			}
			output, status := runExplicitConfigChild(t, tc.path, "observe")
			const marker = "CONFIG_OBSERVATION:"
			start := strings.LastIndex(output, marker)
			if start < 0 {
				t.Fatalf("missing observer exit%d: %s", status, output)
			}
			var got explicitConfigObservation
			if e := json.Unmarshal([]byte(strings.TrimSpace(output[start+len(marker):])), &got); e != nil {
				t.Fatal(e)
			}
			if tc.valid && (!got.BodyCalled || got.ExecuteError != "" || got.Addr != "config-fixture.invalid:7319" || got.DB != 6) {
				t.Fatalf("documented defaults lost: %+v", got)
			}
			if !tc.valid && (got.BodyCalled || got.ExecuteError == "" || !strings.Contains(got.ExecuteError, tc.path)) {
				t.Fatalf("invalid explicit config did not stop command before connection option use: %+v", got)
			}
			t.Logf("case=%s loader_error=%v execute_status=%d observation=%+v", tc.name, parseErr, status, got)
			output, status = runExplicitConfigChild(t, tc.path, "version")
			if tc.valid {
				if status != 0 || !strings.Contains(output, "asynq version") {
					t.Fatalf("valid version rejected: exit%d %s", status, output)
				}
			} else if status != 1 || !strings.Contains(output, tc.path) || strings.Contains(output, "asynq version") {
				t.Fatalf("invalid explicit config did not stop real version: exit%d %s", status, output)
			}
		})
	}
}

func TestCLIExplicitConfigErrorResetContracts(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.yaml")
	b := filepath.Join(dir, "b.yaml")
	missing := filepath.Join(dir, "missing.yaml")
	for path, body := range map[string]string{a: "uri: fixture-a.invalid:7001\ndb: 3\n", b: "uri: fixture-b.invalid:7002\ndb: 4\n"} {
		if e := os.WriteFile(path, []byte(body), 0600); e != nil {
			t.Fatal(e)
		}
	}
	for _, paths := range [][]string{{missing, a, missing}, {a, b}} {
		encoded, _ := json.Marshal(paths)
		output, status := runExplicitConfigChild(t, string(encoded), "repeat")
		const marker = "CONFIG_OBSERVATION:"
		start := strings.LastIndex(output, marker)
		if status != 0 || start < 0 {
			t.Fatalf("repeat child failed exit%d %s", status, output)
		}
		var got []explicitConfigObservation
		if e := json.Unmarshal([]byte(strings.TrimSpace(output[start+len(marker):])), &got); e != nil {
			t.Fatal(e)
		}
		if len(got) != len(paths) {
			t.Fatal("invocation observations lost")
		}
		for n, path := range paths {
			v := got[n]
			if path == missing {
				if v.BodyCalled || v.ExecuteError == "" || !strings.Contains(v.ExecuteError, missing) {
					t.Fatalf("failed invocation reused prior result: %+v", v)
				}
			} else {
				addr, db := "fixture-a.invalid:7001", 3
				if path == b {
					addr, db = "fixture-b.invalid:7002", 4
				}
				if !v.BodyCalled || v.ExecuteError != "" || v.Addr != addr || v.DB != db {
					t.Fatalf("valid invocation poisoned by previous config: %+v", v)
				}
			}
		}
	}
}
