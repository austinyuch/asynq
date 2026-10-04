package cmd

import (
	"bytes"
	"crypto/tls"
	"github.com/austinyuch/asynq"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Cobra flag values and baseline Viper flag bindings are restored. Arbitrary
// Viper config snapshots are not retained; these tests stay sequential.
func isolatedCLI(t *testing.T) {
	t.Helper()
	originalConfig := cfgFile
	viper.Reset()
	viper.SetEnvPrefix("ASYNQ_CLI_CONTRACT")
	rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		original, changed := f.Value.String(), f.Changed
		t.Cleanup(func() { _ = f.Value.Set(original); f.Changed = changed })
		_ = viper.BindPFlag(f.Name, f)
	})
	t.Cleanup(func() {
		cfgFile = originalConfig
		viper.Reset()
		rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
			if err := viper.BindPFlag(f.Name, f); err != nil {
				t.Errorf("restore Viper flag binding %s: %v", f.Name, err)
			}
		})
	})
}
func setCLIFlag(t *testing.T, cmd *cobra.Command, name, value string) {
	t.Helper()
	f := cmd.Flags().Lookup(name)
	if f == nil {
		t.Fatalf("missing flag %s", name)
	}
	original, changed := f.Value.String(), f.Changed
	t.Cleanup(func() { _ = f.Value.Set(original); f.Changed = changed })
	if err := cmd.Flags().Set(name, value); err != nil {
		t.Fatal(err)
	}
}
func captureCLI(t *testing.T, fn func()) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = f
	defer func() { os.Stdout = original; _ = f.Close() }()
	fn()
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
func TestExplicitConfigAndFlagPrecedence(t *testing.T) {
	isolatedCLI(t)
	cfgFile = filepath.Join(t.TempDir(), "asynq.yaml")
	if err := os.WriteFile(cfgFile, []byte("uri: config.invalid:7000\ndb: 8\nusername: fixture-user\npassword: fixture-password\ntls: true\ntls_server: redis.example\n"), 0600); err != nil {
		t.Fatal(err)
	}
	output := captureCLI(t, initConfig)
	if !strings.Contains(output, cfgFile) {
		t.Fatalf("missing config attribution: %q", output)
	}
	opt := getRedisConnOpt().(asynq.RedisClientOpt)
	if opt.Addr != "config.invalid:7000" || opt.DB != 8 || opt.Username != "fixture-user" || opt.Password != "fixture-password" {
		t.Fatalf("config values lost: %+v", opt)
	}
	if opt.TLSConfig == nil || opt.TLSConfig.ServerName != "redis.example" {
		t.Fatal("config TLS server lost")
	}
	f := rootCmd.PersistentFlags().Lookup("uri")
	if err := f.Value.Set("flag.invalid:9000"); err != nil {
		t.Fatal(err)
	}
	f.Changed = true
	if got := getRedisConnOpt().(asynq.RedisClientOpt).Addr; got != "flag.invalid:9000" {
		t.Fatalf("flag must override config: %s", got)
	}
}
func TestRedisTLSConnectionContracts(t *testing.T) {
	for _, tc := range []struct {
		name     string
		tls      bool
		server   string
		insecure bool
		enabled  bool
	}{
		{name: "plain"}, {name: "insecure alone", insecure: true}, {name: "verified TLS", tls: true, enabled: true},
		{name: "server implies TLS", server: "redis.example", enabled: true}, {name: "explicit skip verify", tls: true, insecure: true, enabled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolatedCLI(t)
			viper.Set("tls", tc.tls)
			viper.Set("tls_server", tc.server)
			viper.Set("insecure", tc.insecure)
			cfg := getTLSConfig()
			if !tc.enabled {
				if cfg != nil {
					t.Fatal("TLS enabled unexpectedly")
				}
				return
			}
			if cfg == nil || cfg.MinVersion != tls.VersionTLS12 || cfg.ServerName != tc.server || cfg.InsecureSkipVerify != tc.insecure {
				t.Fatalf("incorrect TLS policy: %+v", cfg)
			}
		})
	}
	t.Run("cluster credentials and addresses", func(t *testing.T) {
		isolatedCLI(t)
		viper.Set("cluster", true)
		viper.Set("cluster_addrs", "first.invalid:7000,second.invalid:7001")
		viper.Set("username", "user")
		viper.Set("password", "pass")
		got := getRedisConnOpt().(asynq.RedisClusterClientOpt)
		if strings.Join(got.Addrs, ",") != "first.invalid:7000,second.invalid:7001" || got.Username != "user" || got.Password != "pass" {
			t.Fatalf("cluster config: %+v", got)
		}
	})
}
func TestCommandHelpAndSuggestions(t *testing.T) {
	var out bytes.Buffer
	old := rootCmd.OutOrStdout()
	rootCmd.SetOut(&out)
	t.Cleanup(func() { rootCmd.SetOut(old) })
	rootHelpFunc(rootCmd, nil)
	for _, want := range []string{"USAGE", "COMMANDS", "task:", "queue:", "--config", "--tls_server", "EXAMPLES", "LEARN MORE"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("help missing %q", want)
		}
	}
	if strings.Contains(out.String(), "version:") {
		t.Error("hidden command leaked")
	}
	out.Reset()
	oldTaskOut := taskCmd.OutOrStdout()
	taskCmd.SetOut(&out)
	t.Cleanup(func() { taskCmd.SetOut(oldTaskOut) })
	rootHelpFunc(taskCmd, []string{"task", "lst"})
	for _, want := range []string{`unknown command "lst" for "asynq task"`, "Did you mean this?", "list", "Available commands:"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("suggestion missing %q in %s", want, out.String())
		}
	}
}
func TestCLIFormattingContracts(t *testing.T) {
	if got := formatQueues(map[string]int{"z": 1, "b": 5, "a": 5}); got != "a:5 b:5 z:1" {
		t.Fatalf("priority/name ordering: %s", got)
	}
	if got := formatQueues(nil); got != "" {
		t.Fatal(got)
	}
	for _, tc := range []struct {
		input []byte
		want  string
	}{
		{[]byte("佇列 payload"), "佇列 payload"}, {nil, "non-printable bytes"}, {[]byte("  "), "non-printable bytes"}, {[]byte{0xff}, "non-printable bytes"}, {[]byte("line\nbreak"), "non-printable bytes"},
	} {
		if got := sprintBytes(tc.input); got != tc.want {
			t.Errorf("payload %q: %q", tc.input, got)
		}
	}
	if capitalize("佇列") != "佇列" || capitalize("task") != "Task" || capitalize("") != "" {
		t.Error("capitalization changed content")
	}
	if got := indent("a\nb\n", 2); got != "  a\n  b\n" {
		t.Fatal(got)
	}
	if indent("", 2) != "" {
		t.Fatal("empty indent")
	}
	if formatNextProcessAt(time.Time{}) != "n/a" || formatNextProcessAt(time.Unix(0, 0)) != "n/a" || formatNextProcessAt(time.Now().Add(-time.Hour)) != "now" {
		t.Error("next process sentinel contract")
	}
	if formatPastTime(time.Time{}) != "" {
		t.Error("zero past time")
	}
	ts := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	if formatPastTime(ts) != ts.Format(time.UnixDate) {
		t.Error("timestamp format")
	}
	output := captureCLI(t, func() {
		printTable([]string{"ID", "Payload"}, func(w io.Writer, _ string) { _, _ = io.WriteString(w, "fixture  value\n") })
	})
	if !strings.Contains(output, "ID") || !strings.Contains(output, "Payload") || !strings.Contains(output, "fixture  value") {
		t.Fatal(output)
	}
}
func TestTaskTimeParsingContracts(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("duration", "", "")
	cmd.Flags().String("time", "", "")
	for _, tc := range []struct {
		input string
		want  time.Duration
	}{{"0s", 0}, {"1h30m", 90 * time.Minute}, {"-1s", -time.Second}} {
		_ = cmd.Flags().Set("duration", tc.input)
		got, err := getDuration(cmd, "duration")
		if err != nil || got != tc.want {
			t.Fatalf("%s: %v %v", tc.input, got, err)
		}
	}
	_ = cmd.Flags().Set("duration", "nonsense")
	if _, err := getDuration(cmd, "duration"); err == nil {
		t.Error("invalid duration accepted")
	}
	if _, err := getDuration(cmd, "missing"); err == nil {
		t.Error("missing duration flag accepted")
	}
	_ = cmd.Flags().Set("time", "2030-01-01T01:02:03+08:00")
	got, err := getTime(cmd, "time")
	if err != nil || got.UTC().Format(time.RFC3339) != "2029-12-31T17:02:03Z" {
		t.Fatalf("offset timestamp: %v %v", got, err)
	}
	_ = cmd.Flags().Set("time", "2030-01-01")
	if _, err := getTime(cmd, "time"); err == nil {
		t.Error("non-RFC3339 accepted")
	}
	if _, err := getTime(cmd, "missing"); err == nil {
		t.Error("missing time flag accepted")
	}
}
func TestTaskEnqueueRejectsInvalidTimeBeforeRedis(t *testing.T) {
	for _, flag := range []string{"timeout", "deadline", "unique", "process_at", "process_in", "retention"} {
		t.Run(flag, func(t *testing.T) {
			setCLIFlag(t, taskEnqueueCmd, "type_name", "fixture")
			setCLIFlag(t, taskEnqueueCmd, "payload", "{}")
			setCLIFlag(t, taskEnqueueCmd, flag, "not-a-time")
			if err := taskEnqueue(taskEnqueueCmd, nil); err == nil || !strings.Contains(err.Error(), "not-a-time") {
				t.Fatalf("expected parsing error naming invalid input, got %v", err)
			}
		})
	}
}

// Execute's os.Exit path runs in an isolated copy of the Go test binary.
func TestCLIProcess(t *testing.T) {
	if os.Getenv("ASYNQ_CLI_CONTRACT_CHILD") != "1" {
		return
	}
	sep := -1
	for i, arg := range os.Args {
		if arg == "--" {
			sep = i
			break
		}
	}
	if sep < 0 {
		t.Fatal("missing child command")
	}
	viper.SetEnvPrefix("ASYNQ_CLI_CONTRACT")
	rootCmd.SetArgs(os.Args[sep+1:])
	Execute()
	os.Exit(0)
}
func runCLIProcess(t *testing.T, args ...string) (string, int) {
	t.Helper()
	config := filepath.Join(t.TempDir(), "empty.yaml")
	if err := os.WriteFile(config, []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	argv := append([]string{"-test.run=^TestCLIProcess$", "--", "--config", config}, args...)
	child := exec.Command(exe, argv...)
	child.Env = append(os.Environ(), "ASYNQ_CLI_CONTRACT_CHILD=1")
	output, err := child.CombinedOutput()
	if err == nil {
		return string(output), 0
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return string(output), exit.ExitCode()
	}
	t.Fatal(err)
	return "", -1
}
func TestCLIExitAndValidationContracts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		args   []string
		status int
		want   string
	}{
		{"version", []string{"version"}, 0, "asynq version"}, {"required queue", []string{"task", "list", "--state=pending"}, 1, `required flag(s) "queue" not set`},
		{"invalid state", []string{"task", "list", "--queue=fixture", "--state=unknown"}, 1, `state="unknown" is not supported`},
		{"aggregating group", []string{"task", "list", "--queue=fixture", "--state=aggregating"}, 1, "flag --group is required"},
		{"invalid refresh", []string{"dash", "--refresh=500ms"}, 1, "--refresh cannot be less than 1s"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, status := runCLIProcess(t, tc.args...)
			if status != tc.status || !strings.Contains(out, tc.want) {
				t.Fatalf("status=%d output=%s", status, out)
			}
		})
	}
}

// Child cleanup must leave production defaults bound and discard its override.
func TestIsolatedCLICleanupRestoresFlagBindings(t *testing.T) {
	before := getRedisConnOpt().(asynq.RedisClientOpt)
	wantAddr := rootCmd.PersistentFlags().Lookup("uri").Value.String()
	if before.Addr != wantAddr {
		t.Fatalf("baseline URI binding missing: got %q want %q", before.Addr, wantAddr)
	}
	t.Run("temporary override", func(t *testing.T) {
		isolatedCLI(t)
		flag := rootCmd.PersistentFlags().Lookup("uri")
		if err := flag.Value.Set("temporary.invalid:9000"); err != nil {
			t.Fatal(err)
		}
		flag.Changed = true
		viper.Set("uri", "viper-override.invalid:8000")
		viper.Set("db", 12)
		if got := getRedisConnOpt().(asynq.RedisClientOpt); got.Addr != "viper-override.invalid:8000" || got.DB != 12 {
			t.Fatalf("child override not applied: %+v", got)
		}
	})
	after := getRedisConnOpt().(asynq.RedisClientOpt)
	if after.Addr != wantAddr || after.DB != before.DB || after.Username != before.Username || after.Password != before.Password {
		t.Fatalf("cleanup lost default bindings or leaked child config: before=%+v after=%+v", before, after)
	}
}
