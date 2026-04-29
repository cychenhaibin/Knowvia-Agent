package store

import (
	"context"
	"testing"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func testMirrorTaskContract(t *testing.T, s contractStore) {
	t.Helper()

	ctx := context.Background()
	taskRunningCompletedAt := contractTime(143)
	taskPendingEarly := domain.MirrorTask{
		ID:          "mirror-task-pending-early",
		Kind:        domain.MirrorTaskSkillUpsert,
		UserID:      "user-contract-mirror",
		ResourceID:  "resource-a",
		Status:      domain.MirrorTaskPending,
		Payload:     `{"kind":"skill"}`,
		Attempts:    1,
		LastError:   "temporary error",
		NextRetryAt: contractTime(141),
		CreatedAt:   contractTime(140),
		UpdatedAt:   contractTime(140),
	}
	taskPendingLate := domain.MirrorTask{
		ID:          "mirror-task-pending-late",
		Kind:        domain.MirrorTaskKnowledgeUpsert,
		UserID:      "user-contract-mirror",
		ResourceID:  "resource-b",
		Status:      domain.MirrorTaskPending,
		Payload:     `{"kind":"knowledge"}`,
		Attempts:    0,
		LastError:   "",
		NextRetryAt: contractTime(142),
		CreatedAt:   contractTime(141),
		UpdatedAt:   contractTime(141),
	}
	taskRunning := domain.MirrorTask{
		ID:          "mirror-task-running",
		Kind:        domain.MirrorTaskKnowledgeDelete,
		UserID:      "user-contract-mirror",
		ResourceID:  "resource-c",
		Status:      domain.MirrorTaskRunning,
		Payload:     `{"kind":"knowledge-delete"}`,
		Attempts:    2,
		LastError:   "",
		NextRetryAt: contractTime(140),
		CreatedAt:   contractTime(139),
		UpdatedAt:   contractTime(143),
		CompletedAt: &taskRunningCompletedAt,
	}
	for _, task := range []domain.MirrorTask{taskPendingEarly, taskPendingLate, taskRunning} {
		if err := s.CreateMirrorTask(ctx, task); err != nil {
			t.Fatalf("CreateMirrorTask %s failed: %v", task.ID, err)
		}
	}

	gotPendingEarly, err := s.GetMirrorTask(ctx, taskPendingEarly.ID)
	if err != nil {
		t.Fatalf("GetMirrorTask failed: %v", err)
	}
	assertDeepEqual(t, "mirror task by id", gotPendingEarly, taskPendingEarly)

	taskPendingEarly.Attempts = 3
	taskPendingEarly.LastError = "retrying"
	taskPendingEarly.UpdatedAt = contractTime(144)
	if err := s.UpdateMirrorTask(ctx, taskPendingEarly); err != nil {
		t.Fatalf("UpdateMirrorTask failed: %v", err)
	}
	gotUpdatedPendingEarly, err := s.GetMirrorTask(ctx, taskPendingEarly.ID)
	if err != nil {
		t.Fatalf("GetMirrorTask after update failed: %v", err)
	}
	assertDeepEqual(t, "updated mirror task", gotUpdatedPendingEarly, taskPendingEarly)

	listedDueTasks, err := s.ListDueMirrorTasks(ctx, contractTime(142), 10)
	if err != nil {
		t.Fatalf("ListDueMirrorTasks failed: %v", err)
	}
	assertDeepEqual(t, "mirror due tasks", listedDueTasks, []domain.MirrorTask{taskPendingEarly, taskPendingLate})

	listedLimitedTasks, err := s.ListDueMirrorTasks(ctx, contractTime(142), 1)
	if err != nil {
		t.Fatalf("ListDueMirrorTasks with limit failed: %v", err)
	}
	assertDeepEqual(t, "mirror due tasks limited", listedLimitedTasks, []domain.MirrorTask{taskPendingEarly})
}
