package agent

import (
	"context"
	"errors"
	"sync"
	"time"
)

const (
	dataCleanupJobStatusRunning = "running"
	dataCleanupJobStatusSuccess = "success"
	dataCleanupJobStatusFailed  = "failed"

	dataCleanupJobTTL = time.Hour
)

type dataCleanupJobSnapshot struct {
	JobID   string
	Module  string
	Status  string
	Current string
	Done    int
	Total   int
	Deleted int64
	Seq     uint64
	Error   string
}

type dataCleanupJob struct {
	jobID  string
	module string

	mu        sync.Mutex
	status    string
	current   string
	done      int
	total     int
	deleted   int64
	seq       uint64
	err       string
	updatedAt time.Time
	expiresAt time.Time

	ctx    context.Context
	cancel context.CancelFunc
}

func (j *dataCleanupJob) snapshot() dataCleanupJobSnapshot {
	j.mu.Lock()
	defer j.mu.Unlock()
	return dataCleanupJobSnapshot{
		JobID:   j.jobID,
		Module:  j.module,
		Status:  j.status,
		Current: j.current,
		Done:    j.done,
		Total:   j.total,
		Deleted: j.deleted,
		Seq:     j.seq,
		Error:   j.err,
	}
}

func (j *dataCleanupJob) touchLocked(now time.Time) {
	j.seq++
	j.updatedAt = now
}

func (j *dataCleanupJob) setCurrent(current string) {
	now := time.Now()
	j.mu.Lock()
	j.current = current
	j.touchLocked(now)
	j.mu.Unlock()
}

func (j *dataCleanupJob) addDeleted(delta int64) {
	if delta <= 0 {
		return
	}
	now := time.Now()
	j.mu.Lock()
	j.deleted += delta
	j.touchLocked(now)
	j.mu.Unlock()
}

func (j *dataCleanupJob) markItemDone() {
	now := time.Now()
	j.mu.Lock()
	j.done++
	j.touchLocked(now)
	j.mu.Unlock()
}

func (j *dataCleanupJob) markItemDoneWithDeleted(delta int64) {
	now := time.Now()
	j.mu.Lock()
	if delta > 0 {
		j.deleted += delta
	}
	j.done++
	j.touchLocked(now)
	j.mu.Unlock()
}

func (j *dataCleanupJob) finalize(err error) {
	now := time.Now()
	j.mu.Lock()
	defer j.mu.Unlock()

	if err != nil {
		j.status = dataCleanupJobStatusFailed
		j.err = err.Error()
	} else {
		j.status = dataCleanupJobStatusSuccess
		j.err = ""
	}
	j.expiresAt = now.Add(dataCleanupJobTTL)
	j.touchLocked(now)
}

func (j *dataCleanupJob) expired(now time.Time) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.status == dataCleanupJobStatusRunning {
		return false
	}
	if j.expiresAt.IsZero() {
		return false
	}
	return now.After(j.expiresAt)
}

type dataCleanupJobManager struct {
	mu   sync.Mutex
	jobs map[string]*dataCleanupJob
}

func newDataCleanupJobManager() *dataCleanupJobManager {
	m := &dataCleanupJobManager{
		jobs: make(map[string]*dataCleanupJob),
	}
	go m.janitor()
	return m
}

func (m *dataCleanupJobManager) janitor() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for now := range ticker.C {
		m.mu.Lock()
		for id, job := range m.jobs {
			if job.expired(now) {
				delete(m.jobs, id)
			}
		}
		m.mu.Unlock()
	}
}

func (m *dataCleanupJobManager) get(jobID string) (*dataCleanupJob, bool) {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[jobID]
	if !ok {
		return nil, false
	}
	if job.expired(now) {
		delete(m.jobs, jobID)
		return nil, false
	}
	return job, true
}

func (m *dataCleanupJobManager) Snapshot(jobID string) (dataCleanupJobSnapshot, error) {
	job, ok := m.get(jobID)
	if !ok {
		return dataCleanupJobSnapshot{}, errors.New("job not found")
	}
	return job.snapshot(), nil
}

func (m *dataCleanupJobManager) Start(
	jobID string,
	module string,
	total int,
	timeout time.Duration,
	run func(ctx context.Context, job *dataCleanupJob) error,
) (dataCleanupJobSnapshot, error) {
	if jobID == "" {
		return dataCleanupJobSnapshot{}, errors.New("jobId is required")
	}
	if module == "" {
		return dataCleanupJobSnapshot{}, errors.New("module is required")
	}
	if total < 0 {
		return dataCleanupJobSnapshot{}, errors.New("total must be >= 0")
	}

	// 幂等：如果 job 已存在且未过期，直接返回当前快照，不重复启动。
	if existing, ok := m.get(jobID); ok {
		return existing.snapshot(), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	job := &dataCleanupJob{
		jobID:     jobID,
		module:    module,
		status:    dataCleanupJobStatusRunning,
		current:   "",
		done:      0,
		total:     total,
		deleted:   0,
		seq:       1,
		err:       "",
		updatedAt: time.Now(),
		ctx:       ctx,
		cancel:    cancel,
	}

	m.mu.Lock()
	// 竞争场景：在 get 与这里之间可能已有并发 Start。
	if existing, ok := m.jobs[jobID]; ok {
		m.mu.Unlock()
		cancel()
		return existing.snapshot(), nil
	}
	m.jobs[jobID] = job
	m.mu.Unlock()

	go func() {
		defer cancel()
		err := run(ctx, job)
		job.finalize(err)
	}()

	return job.snapshot(), nil
}
