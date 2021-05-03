package middleware_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/influxdata/influxdb/v2"
	"github.com/influxdata/influxdb/v2/kit/platform"
	"github.com/influxdata/influxdb/v2/mock"
	"github.com/influxdata/influxdb/v2/notification/check"
	"github.com/influxdata/influxdb/v2/notification/rule"
	"github.com/influxdata/influxdb/v2/task/backend/middleware"
	"github.com/influxdata/influxdb/v2/task/taskmodel"
)

type pipingCoordinator struct {
	err             error
	taskCreatedPipe chan *taskmodel.Task
	taskUpdatedPipe chan *taskmodel.Task
	taskDeletedPipe chan platform.ID
}

func (p *pipingCoordinator) taskCreatedChan() <-chan *taskmodel.Task {
	if p.taskCreatedPipe == nil {
		p.taskCreatedPipe = make(chan *taskmodel.Task, 1)
	}
	return p.taskCreatedPipe
}
func (p *pipingCoordinator) taskUpdatedChan() <-chan *taskmodel.Task {
	if p.taskUpdatedPipe == nil {
		p.taskUpdatedPipe = make(chan *taskmodel.Task, 1)
	}
	return p.taskUpdatedPipe
}
func (p *pipingCoordinator) taskDeletedChan() <-chan platform.ID {
	if p.taskDeletedPipe == nil {
		p.taskDeletedPipe = make(chan platform.ID, 1)
	}
	return p.taskDeletedPipe
}

func (p *pipingCoordinator) TaskCreated(_ context.Context, t *taskmodel.Task) error {
	if p.taskCreatedPipe != nil {
		p.taskCreatedPipe <- t
	}
	return p.err
}
func (p *pipingCoordinator) TaskUpdated(_ context.Context, from, to *taskmodel.Task) error {
	if p.taskUpdatedPipe != nil {
		p.taskUpdatedPipe <- to
	}
	return p.err
}
func (p *pipingCoordinator) TaskDeleted(_ context.Context, id platform.ID) error {
	if p.taskDeletedPipe != nil {
		p.taskDeletedPipe <- id
	}
	return p.err
}
func (p *pipingCoordinator) RunCancelled(ctx context.Context, runID platform.ID) error {
	return p.err
}
func (p *pipingCoordinator) RunRetried(ctx context.Context, task *taskmodel.Task, run *taskmodel.Run) error {
	return p.err
}
func (p *pipingCoordinator) RunForced(ctx context.Context, task *taskmodel.Task, run *taskmodel.Run) error {
	return p.err
}

type mockedSvc struct {
	taskSvc           *mock.TaskService
	checkSvc          *mock.CheckService
	notificationSvc   *mock.NotificationRuleStore
	pipingCoordinator *pipingCoordinator
}

func newMockServices() mockedSvc {
	return mockedSvc{
		taskSvc: &mock.TaskService{
			FindTaskByIDFn: func(_ context.Context, id platform.ID) (*taskmodel.Task, error) { return &taskmodel.Task{ID: id}, nil },
			CreateTaskFn:   func(context.Context, taskmodel.TaskCreate) (*taskmodel.Task, error) { return &taskmodel.Task{ID: 1}, nil },
			UpdateTaskFn: func(_ context.Context, id platform.ID, _ taskmodel.TaskUpdate) (*taskmodel.Task, error) {
				return &taskmodel.Task{ID: id}, nil
			},
			DeleteTaskFn: func(context.Context, platform.ID) error { return nil },
		},
		checkSvc: &mock.CheckService{
			FindCheckByIDFn: func(_ context.Context, id platform.ID) (influxdb.Check, error) {
				c := &check.Deadman{}
				c.SetID(id)
				return c, nil
			},
			CreateCheckFn: func(context.Context, influxdb.CheckCreate, platform.ID) error { return nil },
			UpdateCheckFn: func(_ context.Context, _ platform.ID, c influxdb.CheckCreate) (influxdb.Check, error) { return c, nil },
			PatchCheckFn: func(_ context.Context, id platform.ID, _ influxdb.CheckUpdate) (influxdb.Check, error) {
				c := &check.Deadman{}
				c.SetID(id)
				return c, nil
			},
			DeleteCheckFn: func(context.Context, platform.ID) error { return nil },
		},
		notificationSvc: &mock.NotificationRuleStore{
			FindNotificationRuleByIDF: func(_ context.Context, id platform.ID) (influxdb.NotificationRule, error) {
				c := &rule.HTTP{}
				c.SetID(id)
				return c, nil
			},
			CreateNotificationRuleF: func(context.Context, influxdb.NotificationRuleCreate, platform.ID) error { return nil },
			UpdateNotificationRuleF: func(_ context.Context, _ platform.ID, c influxdb.NotificationRuleCreate, _ platform.ID) (influxdb.NotificationRule, error) {
				return c, nil
			},
			PatchNotificationRuleF: func(_ context.Context, id platform.ID, _ influxdb.NotificationRuleUpdate) (influxdb.NotificationRule, error) {
				c := &rule.HTTP{}
				c.SetID(id)
				return c, nil
			},
			DeleteNotificationRuleF: func(context.Context, platform.ID) error { return nil },
		},
		pipingCoordinator: &pipingCoordinator{},
	}
}

func newCheckSvcStack() (mockedSvc, *middleware.CoordinatingCheckService) {
	msvcs := newMockServices()
	return msvcs, middleware.NewCheckService(msvcs.checkSvc, msvcs.taskSvc, msvcs.pipingCoordinator)
}

func TestCheckCreate(t *testing.T) {
	mocks, checkService := newCheckSvcStack()
	ch := mocks.pipingCoordinator.taskCreatedChan()

	check := &check.Deadman{}
	check.SetTaskID(4)

	cc := influxdb.CheckCreate{
		Check:  check,
		Status: influxdb.Active,
	}

	err := checkService.CreateCheck(context.Background(), cc, 1)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case task := <-ch:
		if task.ID != check.GetTaskID() {
			t.Fatalf("task sent to coordinator doesn't match expected")
		}
	default:
		t.Fatal("didn't receive task")
	}

	mocks.pipingCoordinator.err = fmt.Errorf("bad")
	mocks.checkSvc.DeleteCheckFn = func(context.Context, platform.ID) error { return fmt.Errorf("AARGH") }

	err = checkService.CreateCheck(context.Background(), cc, 1)
	if err.Error() != "schedule task failed: bad\n\tcleanup also failed: AARGH" {
		t.Fatal(err)
	}
}

func TestCheckUpdateFromInactive(t *testing.T) {
	mocks, checkService := newCheckSvcStack()
	latest := time.Now().UTC()
	checkService.Now = func() time.Time {
		return latest
	}
	ch := mocks.pipingCoordinator.taskUpdatedChan()

	mocks.checkSvc.UpdateCheckFn = func(_ context.Context, _ platform.ID, c influxdb.CheckCreate) (influxdb.Check, error) {
		c.SetTaskID(10)
		c.SetUpdatedAt(latest.Add(-20 * time.Hour))
		return c, nil
	}

	mocks.checkSvc.PatchCheckFn = func(_ context.Context, _ platform.ID, c influxdb.CheckUpdate) (influxdb.Check, error) {
		ic := &check.Deadman{}
		ic.SetTaskID(10)
		ic.SetUpdatedAt(latest.Add(-20 * time.Hour))
		return ic, nil
	}

	mocks.checkSvc.FindCheckByIDFn = func(_ context.Context, id platform.ID) (influxdb.Check, error) {
		c := &check.Deadman{}
		c.SetID(id)
		c.SetTaskID(1)
		return c, nil
	}

	mocks.taskSvc.FindTaskByIDFn = func(_ context.Context, id platform.ID) (*taskmodel.Task, error) {
		if id == 1 {
			return &taskmodel.Task{ID: id, Status: string(taskmodel.TaskInactive)}, nil
		} else if id == 10 {
			return &taskmodel.Task{ID: id, Status: string(taskmodel.TaskActive)}, nil
		}
		return &taskmodel.Task{ID: id}, nil
	}

	deadman := &check.Deadman{}
	deadman.SetTaskID(10)

	cc := influxdb.CheckCreate{
		Check:  deadman,
		Status: influxdb.Active,
	}

	thecheck, err := checkService.UpdateCheck(context.Background(), 1, cc)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case task := <-ch:
		if task.ID != thecheck.GetTaskID() {
			t.Fatalf("task sent to coordinator doesn't match expected")
		}
		if task.LatestCompleted != latest {
			t.Fatalf("update returned incorrect LatestCompleted, expected %s got %s, or ", latest.Format(time.RFC3339), task.LatestCompleted)
		}
	default:
		t.Fatal("didn't receive task")
	}

	action := influxdb.Active
	thecheck, err = checkService.PatchCheck(context.Background(), 1, influxdb.CheckUpdate{Status: &action})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case task := <-ch:
		if task.ID != thecheck.GetTaskID() {
			t.Fatalf("task sent to coordinator doesn't match expected")
		}
		if task.LatestCompleted != latest {
			t.Fatalf("update returned incorrect LatestCompleted, expected %s got %s, or ", latest.Format(time.RFC3339), task.LatestCompleted)
		}
	default:
		t.Fatal("didn't receive task")
	}

}

func TestCheckUpdate(t *testing.T) {
	mocks, checkService := newCheckSvcStack()
	ch := mocks.pipingCoordinator.taskUpdatedChan()

	mocks.checkSvc.UpdateCheckFn = func(_ context.Context, _ platform.ID, c influxdb.CheckCreate) (influxdb.Check, error) {
		c.SetTaskID(10)
		return c, nil
	}

	deadman := &check.Deadman{}
	deadman.SetTaskID(4)

	cc := influxdb.CheckCreate{
		Check:  deadman,
		Status: influxdb.Active,
	}

	check, err := checkService.UpdateCheck(context.Background(), 1, cc)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case task := <-ch:
		if task.ID != check.GetTaskID() {
			t.Fatalf("task sent to coordinator doesn't match expected")
		}
	default:
		t.Fatal("didn't receive task")
	}
}

func TestCheckPatch(t *testing.T) {
	mocks, checkService := newCheckSvcStack()
	ch := mocks.pipingCoordinator.taskUpdatedChan()

	deadman := &check.Deadman{}
	deadman.SetTaskID(4)

	mocks.checkSvc.PatchCheckFn = func(context.Context, platform.ID, influxdb.CheckUpdate) (influxdb.Check, error) {
		return deadman, nil
	}

	check, err := checkService.PatchCheck(context.Background(), 1, influxdb.CheckUpdate{})
	if err != nil {
		t.Fatal(err)
	}

	select {
	case task := <-ch:
		if task.ID != check.GetTaskID() {
			t.Fatalf("task sent to coordinator doesn't match expected")
		}
	default:
		t.Fatal("didn't receive task")
	}
}

func TestCheckDelete(t *testing.T) {
	mocks, checkService := newCheckSvcStack()
	ch := mocks.pipingCoordinator.taskDeletedChan()

	mocks.checkSvc.FindCheckByIDFn = func(_ context.Context, id platform.ID) (influxdb.Check, error) {
		c := &check.Deadman{}
		c.SetID(id)
		c.SetTaskID(21)
		return c, nil
	}

	err := checkService.DeleteCheck(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case id := <-ch:
		if id != platform.ID(21) {
			t.Fatalf("task sent to coordinator doesn't match expected")
		}
	default:
		t.Fatal("didn't receive task")
	}
}
