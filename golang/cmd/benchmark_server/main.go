package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/jmoiron/sqlx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	xsuportal "github.com/isucon/isucon10-final/webapp/golang"
	"github.com/isucon/isucon10-final/webapp/golang/proto/xsuportal/resources"
	"github.com/isucon/isucon10-final/webapp/golang/proto/xsuportal/services/bench"
	"github.com/isucon/isucon10-final/webapp/golang/util"
)

var db *sqlx.DB

type benchmarkQueueService struct {
}

func (b *benchmarkQueueService) Svc() *bench.BenchmarkQueueService {
	return &bench.BenchmarkQueueService{
		ReceiveBenchmarkJob: b.ReceiveBenchmarkJob,
	}
}

func (b *benchmarkQueueService) ReceiveBenchmarkJob(ctx context.Context, req *bench.ReceiveBenchmarkJobRequest) (*bench.ReceiveBenchmarkJobResponse, error) {
	var jobHandle *bench.ReceiveBenchmarkJobResponse_JobHandle
	var job *xsuportal.BenchmarkJob

	err := func() error {
		for {
			j, err := fetchBenchmarkJob(db, req.TeamId)
			if err != nil {
				return fmt.Errorf("poll benchmark job: %w", err)
			}
			if j == nil {
				time.Sleep(time.Millisecond * 1000)
				return nil
			}
			job = j

			randomBytes := make([]byte, 16)
			_, err = rand.Read(randomBytes)
			if err != nil {
				return fmt.Errorf("read random: %w", err)
			}
			handle := base64.StdEncoding.EncodeToString(randomBytes)

			// 楽観ロック
			r, err := db.Exec(
				"UPDATE `benchmark_jobs` SET `status` = ?, `handle` = ? WHERE `id` = ? AND `status` = ?",
				resources.BenchmarkJob_SENT,
				handle,
				job.ID,
				resources.BenchmarkJob_PENDING,
			)
			if err != nil {
				return fmt.Errorf("update benchmark job status: %w", err)
			}
			if rows, _ := r.RowsAffected(); rows == 0 {
				continue
			}

			var contestStartsAt time.Time
			err = db.Get(&contestStartsAt, "SELECT `contest_starts_at` FROM `contest_config` LIMIT 1")
			if err != nil {
				return fmt.Errorf("get contest starts at: %w", err)
			}

			jobHandle = &bench.ReceiveBenchmarkJobResponse_JobHandle{
				JobId:            job.ID,
				Handle:           handle,
				TargetHostname:   job.TargetHostName,
				ContestStartedAt: timestamppb.New(contestStartsAt),
				JobCreatedAt:     timestamppb.New(job.CreatedAt),
			}
			log.Printf("[DEBUG] Dequeued: job_handle=%+v", jobHandle)

			return nil
		}
	}()
	if err != nil {
		// エラーが起きたらPENDINGに戻しておく
		_, _ = db.Exec(
			"UPDATE `benchmark_jobs` SET `status` = ?, `handle` = '' WHERE `id` = ?",
			resources.BenchmarkJob_PENDING,
			job.ID,
		)

		return nil, fmt.Errorf("fetch queue: %w", err)
	}

	return &bench.ReceiveBenchmarkJobResponse{
		JobHandle: jobHandle,
	}, nil
}

type benchmarkReportService struct {
}

func (b *benchmarkReportService) Svc() *bench.BenchmarkReportService {
	return &bench.BenchmarkReportService{
		ReportBenchmarkResult: b.ReportBenchmarkResult,
	}
}

func (b *benchmarkReportService) ReportBenchmarkResult(srv bench.BenchmarkReport_ReportBenchmarkResultServer) error {
	var notifier xsuportal.Notifier
	for {
		req, err := srv.Recv()
		if err != nil {
			return err
		}
		if req.Result == nil {
			return status.Error(codes.InvalidArgument, "result required")
		}

		err = func() error {
			tx, err := db.Beginx()
			if err != nil {
				return fmt.Errorf("begin tx: %w", err)
			}
			defer tx.Rollback()

			var job xsuportal.BenchmarkJob
			err = tx.Get(
				&job,
				"SELECT * FROM `benchmark_jobs` WHERE `id` = ? AND `handle` = ? LIMIT 1 FOR UPDATE",
				req.JobId,
				req.Handle,
			)
			if err == sql.ErrNoRows {
				log.Printf("[ERROR] Job not found: job_id=%v, handle=%+v", req.JobId, req.Handle)
				return status.Errorf(codes.NotFound, "Job %d not found or handle is wrong", req.JobId)
			}
			if err != nil {
				return fmt.Errorf("get benchmark job: %w", err)
			}
			if req.Result.Finished {
				log.Printf("[DEBUG] %v: save as finished", req.JobId)
				if err := b.saveAsFinished(tx, &job, req); err != nil {
					return err
				}
				if err := tx.Commit(); err != nil {
					return fmt.Errorf("commit tx: %w", err)
				}
				if err := notifier.NotifyBenchmarkJobFinished(db, &job); err != nil {
					return fmt.Errorf("notify benchmark job finished: %w", err)
				}
			} else {
				log.Printf("[DEBUG] %v: save as running", req.JobId)
				if err := b.saveAsRunning(tx, &job, req); err != nil {
					return err
				}
				if err := tx.Commit(); err != nil {
					return fmt.Errorf("commit tx: %w", err)
				}
			}
			return nil
		}()
		if err != nil {
			return err
		}
		err = srv.Send(&bench.ReportBenchmarkResultResponse{
			AckedNonce: req.GetNonce(),
		})
		if err != nil {
			return fmt.Errorf("send report: %w", err)
		}
	}
}

func (b *benchmarkReportService) saveAsFinished(db sqlx.Execer, job *xsuportal.BenchmarkJob, req *bench.ReportBenchmarkResultRequest) error {
	if !job.StartedAt.Valid || job.FinishedAt.Valid {
		return status.Errorf(codes.FailedPrecondition, "Job %v has already finished or has not started yet", req.JobId)
	}
	if req.Result.MarkedAt == nil {
		return status.Errorf(codes.InvalidArgument, "marked_at is required")
	}
	markedAt := req.Result.MarkedAt.AsTime().Round(time.Microsecond)

	result := req.Result
	var raw, deduction sql.NullInt32
	if result.ScoreBreakdown != nil {
		raw.Valid = true
		raw.Int32 = int32(result.ScoreBreakdown.Raw)
		deduction.Valid = true
		deduction.Int32 = int32(result.ScoreBreakdown.Deduction)
	}
	_, err := db.Exec(
		"UPDATE `benchmark_jobs` SET `status` = ?, `score_raw` = ?, `score_deduction` = ?, `passed` = ?, `reason` = ?, `updated_at` = NOW(6), `finished_at` = ? WHERE `id` = ? LIMIT 1",
		resources.BenchmarkJob_FINISHED,
		raw,
		deduction,
		result.Passed,
		result.Reason,
		markedAt,
		req.JobId,
	)
	if err != nil {
		return fmt.Errorf("update benchmark job status: %w", err)
	}
	return nil
}

func (b *benchmarkReportService) saveAsRunning(db sqlx.Execer, job *xsuportal.BenchmarkJob, req *bench.ReportBenchmarkResultRequest) error {
	if req.Result.MarkedAt == nil {
		return status.Errorf(codes.InvalidArgument, "marked_at is required")
	}
	var startedAt time.Time
	if job.StartedAt.Valid {
		startedAt = job.StartedAt.Time
	} else {
		startedAt = req.Result.MarkedAt.AsTime().Round(time.Microsecond)
	}
	_, err := db.Exec(
		"UPDATE `benchmark_jobs` SET `status` = ?, `score_raw` = NULL, `score_deduction` = NULL, `passed` = FALSE, `reason` = NULL, `started_at` = ?, `updated_at` = NOW(6), `finished_at` = NULL WHERE `id` = ? LIMIT 1",
		resources.BenchmarkJob_RUNNING,
		startedAt,
		req.JobId,
	)
	if err != nil {
		return fmt.Errorf("update benchmark job status: %w", err)
	}
	return nil
}

func fetchBenchmarkJob(db sqlx.Queryer, teamId int64) (*xsuportal.BenchmarkJob, error) {
	var job xsuportal.BenchmarkJob
	err := sqlx.Get(
		db,
		&job,
		"SELECT * FROM `benchmark_jobs` WHERE team_id = ? and `status` = ? ORDER BY `id` LIMIT 1",
		teamId,
		resources.BenchmarkJob_PENDING,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get benchmark job: %w", err)
	}

	return &job, nil
}

func main() {
	port := util.GetEnv("PORT", "50051")
	address := ":" + port

	listener, err := net.Listen("tcp", address)
	if err != nil {
		panic(err)
	}
	log.Print("[INFO] listen ", address)

	db, _ = xsuportal.GetDB()
	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(50)

	server := grpc.NewServer()

	queue := &benchmarkQueueService{}
	report := &benchmarkReportService{}

	bench.RegisterBenchmarkQueueService(server, queue.Svc())
	bench.RegisterBenchmarkReportService(server, report.Svc())

	if err := server.Serve(listener); err != nil {
		panic(err)
	}
}
