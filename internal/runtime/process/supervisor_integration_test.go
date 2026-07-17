package process

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/runtime/domain"
)

func TestNativeProcessLifecycleCapturesTreeLogsMetricsAndStops(t *testing.T) {
	port := freePort(t)
	driver, store, project, cancel := nativeTestDriver(t, "tree", port, domain.RestartPolicy{})
	defer cancel()
	executeProcessAction(t, driver, project, domain.ActionStart)
	waitForTCP(t, port)
	observation := waitForProcessState(t, driver, project, domain.StateRunning)
	if observation.Services[0].Process == nil || observation.Services[0].Process.RunID == "" {
		t.Fatalf("observation = %#v", observation)
	}
	waitForCapturedStreams(t, driver, project)
	metrics := &metricSinkFake{}
	if err := driver.StreamMetrics(context.Background(), domain.MetricRequest{Project: project}, metrics); err != nil {
		t.Fatal(err)
	}
	if len(metrics.samples) != 1 || metrics.samples[0].MemoryBytes == 0 {
		t.Fatalf("metrics = %#v", metrics.samples)
	}
	executeProcessAction(t, driver, project, domain.ActionStop)
	waitForProcessState(t, driver, project, domain.StateStopped)
	runs, _ := store.ListProjectRuns(context.Background(), project.ProjectID)
	if len(runs) != 1 || runs[0].EndedAt == nil || runs[0].TerminationReason != "stopped" || len(runs[0].Processes) < 2 {
		t.Fatalf("run = %#v", runs)
	}
}

func TestNativeProcessCorrelatesRunAndLogsToOperation(t *testing.T) {
	port := freePort(t)
	driver, store, project, cancel := nativeTestDriver(t, "tree", port, domain.RestartPolicy{})
	defer cancel()
	plan, err := driver.Plan(context.Background(), domain.PlanRequest{Project: project, Action: domain.ActionStart})
	if err != nil {
		t.Fatal(err)
	}
	plan.OperationID = "op-1"
	if err := driver.Execute(context.Background(), plan, progressSinkFake{}); err != nil {
		t.Fatal(err)
	}
	waitForTCP(t, port)
	waitForCapturedStreams(t, driver, project)
	runs, err := store.ListProjectRuns(context.Background(), project.ProjectID)
	if err != nil || len(runs) != 1 || runs[0].OperationID != "op-1" {
		t.Fatalf("runs = %#v, error = %v", runs, err)
	}
	sink := &logSinkFake{}
	if err := driver.StreamLogs(context.Background(), domain.LogRequest{Project: project, Tail: 100}, sink); err != nil {
		t.Fatal(err)
	}
	for _, entry := range sink.entries {
		if entry.OperationID != "op-1" {
			t.Fatalf("entry = %#v", entry)
		}
	}
	executeProcessAction(t, driver, project, domain.ActionStop)
}

func TestNativeProcessKeepsOrphanedChildManaged(t *testing.T) {
	port := freePort(t)
	driver, store, project, cancel := nativeTestDriver(t, "orphan", port, domain.RestartPolicy{})
	defer cancel()
	executeProcessAction(t, driver, project, domain.ActionStart)
	waitForTCP(t, port)
	time.Sleep(300 * time.Millisecond)
	observation := waitForProcessState(t, driver, project, domain.StateRunning)
	if observation.Services[0].Process == nil {
		t.Fatalf("orphan observation = %#v", observation)
	}
	runs, _ := store.ListProjectRuns(context.Background(), project.ProjectID)
	if len(runs[0].Processes) < 2 {
		t.Fatalf("orphan child fingerprint was not persisted: %#v", runs[0])
	}
	executeProcessAction(t, driver, project, domain.ActionStop)
	waitForProcessState(t, driver, project, domain.StateStopped)
}

func TestNativeProcessReportsCrashAndOptInRestart(t *testing.T) {
	driver, store, project, cancel := nativeTestDriver(t, "crash", 0, domain.RestartPolicy{})
	defer cancel()
	executeProcessAction(t, driver, project, domain.ActionStart)
	observation := waitForProcessState(t, driver, project, domain.StateFailed)
	if observation.Services[0].Process == nil || observation.Services[0].Process.ExitCode == nil || *observation.Services[0].Process.ExitCode != 17 {
		t.Fatalf("crash observation = %#v", observation)
	}
	runs, _ := store.ListProjectRuns(context.Background(), project.ProjectID)
	if runs[0].TerminationReason != "crashed" {
		t.Fatalf("crash run = %#v", runs[0])
	}

	port := freePort(t)
	restartDriver, restartStore, restartProject, restartCancel := nativeTestDriver(t, "crash-once", port, domain.RestartPolicy{Mode: "on-failure", MaxRetries: 1})
	defer restartCancel()
	marker := filepath.Join(t.TempDir(), "crashed")
	restartProject.Process.Processes[0].Environment["MARKER"] = marker
	executeProcessAction(t, restartDriver, restartProject, domain.ActionStart)
	waitForTCP(t, port)
	restarted := waitForProcessState(t, restartDriver, restartProject, domain.StateRunning)
	if restarted.Services[0].Process.RestartCount != 1 {
		t.Fatalf("restart observation = %#v", restarted)
	}
	executeProcessAction(t, restartDriver, restartProject, domain.ActionStop)
	runs, _ = restartStore.ListProjectRuns(context.Background(), restartProject.ProjectID)
	if runs[0].RestartCount != 1 {
		t.Fatalf("restart run = %#v", runs[0])
	}
}

func TestNativeProcessEscalatesWhenGracefulStopIsIgnored(t *testing.T) {
	port := freePort(t)
	driver, store, project, cancel := nativeTestDriver(t, "ignore", port, domain.RestartPolicy{})
	defer cancel()
	project.Process.Processes[0].StopTimeoutSeconds = 1
	executeProcessAction(t, driver, project, domain.ActionStart)
	waitForTCP(t, port)
	executeProcessAction(t, driver, project, domain.ActionStop)
	runs, _ := store.ListProjectRuns(context.Background(), project.ProjectID)
	if runs[0].TerminationReason != "stopped_forced" {
		t.Fatalf("forced run = %#v", runs[0])
	}
}

func TestNativeProcessCancellationRollsBackStartedDependencies(t *testing.T) {
	port := freePort(t)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	driverCtx, driverCancel := context.WithCancel(context.Background())
	defer driverCancel()
	store := newMemoryRunStore()
	driver := newDriver(driverCtx, store, gopsutilInspector{}, &secretResolverFake{})
	root := t.TempDir()
	command := []string{executable, "-test.run=TestNativeProcessHelper", "--", "child"}
	project := processProject(root, []servicePlan{
		{
			service: domain.ServiceDeclaration{ID: "first", RuntimeName: "first"},
			definition: domain.ProcessDefinition{
				ID: "first", Command: command, WorkingDirectory: root, StopTimeoutSeconds: 1,
				Environment: map[string]string{"SWITCHYARD_PROCESS_HELPER": "1", "PORT": strconv.Itoa(port)},
			},
		},
		{
			service: domain.ServiceDeclaration{ID: "second", RuntimeName: "second", Dependencies: []string{"first"}},
			definition: domain.ProcessDefinition{
				ID: "second", Command: command, WorkingDirectory: root, StopTimeoutSeconds: 1,
				Environment: map[string]string{"SWITCHYARD_PROCESS_HELPER": "1", "PORT": strconv.Itoa(freePort(t))},
			},
		},
	})
	operationCtx, operationCancel := context.WithCancel(context.Background())
	plan, err := driver.Plan(operationCtx, domain.PlanRequest{Project: project, Action: domain.ActionStart})
	if err != nil {
		t.Fatal(err)
	}
	err = driver.Execute(operationCtx, plan, cancelAfterStepSink{cancel: operationCancel, step: "process.start.first"})
	if err == nil || operationCtx.Err() == nil {
		t.Fatalf("Execute() error = %v, context = %v", err, operationCtx.Err())
	}
	runs, _ := store.ListProjectRuns(context.Background(), project.ProjectID)
	if len(runs) != 1 || runs[0].EndedAt == nil || runs[0].TerminationReason != "start_cancelled" {
		t.Fatalf("cancelled runs = %#v", runs)
	}
	for _, identity := range runs[0].Processes {
		if _, snapshotErr := (gopsutilInspector{}).Snapshot(context.Background(), identity.PID); snapshotErr == nil {
			t.Fatalf("cancelled PID %d still exists", identity.PID)
		}
	}
}

func TestNativeProcessHelper(_ *testing.T) {
	if os.Getenv("SWITCHYARD_PROCESS_HELPER") != "1" {
		return
	}
	mode := helperArgument()
	switch mode {
	case "tree", "orphan":
		command := exec.Command(os.Args[0], "-test.run=TestNativeProcessHelper", "--", "child")
		command.Env = os.Environ()
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		if err := command.Start(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(10)
		}
		fmt.Println("info helper parent started child")
		fmt.Fprintln(os.Stderr, "warning helper parent stderr")
		if mode == "orphan" {
			time.Sleep(250 * time.Millisecond)
			return
		}
		if err := command.Wait(); err != nil {
			os.Exit(11)
		}
	case "child":
		serveHelper(false)
	case "ignore":
		serveHelper(true)
	case "crash":
		fmt.Fprintln(os.Stderr, "error helper intentional crash")
		os.Exit(17)
	case "crash-once":
		marker := os.Getenv("MARKER")
		if _, err := os.Stat(marker); os.IsNotExist(err) {
			if writeErr := os.WriteFile(marker, []byte("crashed"), 0o600); writeErr != nil {
				os.Exit(18)
			}
			os.Exit(17)
		}
		serveHelper(false)
	default:
		os.Exit(19)
	}
}

func helperArgument() string {
	for index, argument := range os.Args {
		if argument == "--" && index+1 < len(os.Args) {
			return os.Args[index+1]
		}
	}
	return ""
}

func serveHelper(ignoreTermination bool) {
	if ignoreTermination {
		// The forced-termination test proves escalation after this explicit refusal.
		// Windows delivers CTRL_BREAK_EVENT as os.Interrupt, while Unix uses
		// SIGTERM. Ignore both so the fixture exercises the same escalation path.
		ignoreSignal(syscall.SIGTERM, os.Interrupt)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:"+os.Getenv("PORT"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(20)
	}
	defer func() { _ = listener.Close() }()
	fmt.Println("info helper child listening")
	for {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		_ = connection.Close()
	}
}

func nativeTestDriver(t *testing.T, mode string, port int, restart domain.RestartPolicy) (*Driver, *memoryRunStore, domain.ProjectRuntime, context.CancelFunc) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	store := newMemoryRunStore()
	driver := newDriver(ctx, store, gopsutilInspector{}, &secretResolverFake{})
	definition := domain.ProcessDefinition{
		ID: "web", Command: []string{executable, "-test.run=TestNativeProcessHelper", "--", mode},
		WorkingDirectory: t.TempDir(), Environment: map[string]string{
			"SWITCHYARD_PROCESS_HELPER": "1", "PORT": strconv.Itoa(port),
		}, Restart: restart, StopTimeoutSeconds: 2,
	}
	service := domain.ServiceDeclaration{ID: "web", RuntimeName: "web"}
	project := processProject(definition.WorkingDirectory, []servicePlan{{service: service, definition: definition}})
	t.Cleanup(func() {
		plan, planErr := driver.Plan(context.Background(), domain.PlanRequest{Project: project, Action: domain.ActionStop})
		if planErr == nil {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 3*time.Second)
			_ = driver.Execute(cleanupCtx, plan, progressSinkFake{})
			cleanupCancel()
		}
	})
	return driver, store, project, cancel
}

func executeProcessAction(t *testing.T, driver *Driver, project domain.ProjectRuntime, action domain.Action) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	plan, err := driver.Plan(ctx, domain.PlanRequest{Project: project, Action: action})
	if err != nil {
		t.Fatal(err)
	}
	if err := driver.Execute(ctx, plan, progressSinkFake{}); err != nil {
		t.Fatal(err)
	}
}

func waitForProcessState(t *testing.T, driver *Driver, project domain.ProjectRuntime, wanted domain.ProjectState) domain.Observation {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	var observation domain.Observation
	for time.Now().Before(deadline) {
		var err error
		observation, err = driver.Inspect(context.Background(), project)
		if err != nil {
			t.Fatal(err)
		}
		if observation.State == wanted {
			return observation
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("waiting for %s; last observation = %#v", wanted, observation)
	return domain.Observation{}
}

func waitForCapturedStreams(t *testing.T, driver *Driver, project domain.ProjectRuntime) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		sink := &logSinkFake{}
		if err := driver.StreamLogs(context.Background(), domain.LogRequest{Project: project, Tail: 100}, sink); err != nil {
			t.Fatal(err)
		}
		streams := map[string]bool{}
		for _, entry := range sink.entries {
			streams[entry.Stream] = true
			if entry.Source != "process" || entry.RunID == "" {
				t.Fatalf("log identity = %#v", entry)
			}
		}
		if streams["stdout"] && streams["stderr"] {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("stdout and stderr were not both captured")
}

func waitForTCP(t *testing.T, port int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		connection, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(port), 50*time.Millisecond)
		if err == nil {
			_ = connection.Close()
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("port %d did not begin listening", port)
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	return listener.Addr().(*net.TCPAddr).Port
}

func ignoreSignal(signals ...os.Signal) {
	signalChannel := make(chan os.Signal, 1)
	signalNotify(signalChannel, signals...)
}

var signalNotify = func(channel chan<- os.Signal, signals ...os.Signal) {
	// Kept as a seam because signal.Notify cannot be called through a type alias.
	signalNotifyPlatform(channel, signals...)
}

type cancelAfterStepSink struct {
	cancel context.CancelFunc
	step   string
}

func (s cancelAfterStepSink) Step(_ context.Context, name, _, _ string) error {
	if name == s.step {
		s.cancel()
	}
	return nil
}
