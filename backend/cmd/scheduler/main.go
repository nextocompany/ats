// Command scheduler is the single periodic dispatcher. It runs one asynq
// Scheduler that enqueues report-export tasks on REPORT_SCHEDULE_CRON. It must
// run as exactly ONE replica — multiple instances would double-enqueue. The
// worker consumes the tasks it produces.
package main

import (
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/pkg/config"
	"github.com/nexto/hr-ats/pkg/logging"
	"github.com/nexto/hr-ats/pkg/queue"
)

func main() {
	// config.Load requires DB/blob vars even though the scheduler only enqueues to
	// Redis; they are validated but unused here (the worker holds those concerns).
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("config load failed")
	}
	logging.Configure(cfg.IsDevelopment())

	redisOpt, err := queue.RedisOpt(cfg.RedisURL)
	if err != nil {
		log.Fatal().Err(err).Msg("queue redis opt failed")
	}

	scheduler := asynq.NewScheduler(redisOpt, &asynq.SchedulerOpts{})

	// Period is intentionally left empty: asynq.Scheduler enqueues a fixed task
	// copy each tick, so the worker derives the period (ISO week) at run time
	// rather than freezing it to scheduler-startup time.
	task, err := queue.NewExportReportTask(queue.ExportReportPayload{Kind: "weekly"})
	if err != nil {
		log.Fatal().Err(err).Msg("build export task failed")
	}
	entryID, err := scheduler.Register(cfg.ReportScheduleCron, task)
	if err != nil {
		log.Fatal().Err(err).Str("cron", cfg.ReportScheduleCron).Msg("register schedule failed")
	}

	log.Info().Str("cron", cfg.ReportScheduleCron).Str("entry_id", entryID).Msg("scheduler started; report:export registered")

	// Retention sweep (Sprint 7): registered only when explicitly enabled, so a
	// disabled environment never enqueues the destructive PDPA job.
	if cfg.RetentionSweepEnabled {
		sweepTask, err := queue.NewRetentionSweepTask(queue.RetentionSweepPayload{})
		if err != nil {
			log.Fatal().Err(err).Msg("build retention sweep task failed")
		}
		sweepID, err := scheduler.Register(cfg.RetentionSweepCron, sweepTask)
		if err != nil {
			log.Fatal().Err(err).Str("cron", cfg.RetentionSweepCron).Msg("register retention sweep failed")
		}
		log.Info().Str("cron", cfg.RetentionSweepCron).Str("entry_id", sweepID).Msg("scheduler: retention:sweep registered")
	} else {
		log.Info().Msg("scheduler: retention sweep disabled (RETENTION_SWEEP_ENABLED=false)")
	}

	// Auth cleanup (candidate membership): purge expired OTP/session rows. Benign
	// housekeeping → enabled by default (only removes already-dead auth artifacts).
	if cfg.AuthCleanupEnabled {
		cleanupTask, err := queue.NewAuthCleanupTask(queue.AuthCleanupPayload{})
		if err != nil {
			log.Fatal().Err(err).Msg("build auth cleanup task failed")
		}
		cleanupID, err := scheduler.Register(cfg.AuthCleanupCron, cleanupTask)
		if err != nil {
			log.Fatal().Err(err).Str("cron", cfg.AuthCleanupCron).Msg("register auth cleanup failed")
		}
		log.Info().Str("cron", cfg.AuthCleanupCron).Str("entry_id", cleanupID).Msg("scheduler: auth:cleanup registered")
	} else {
		log.Info().Msg("scheduler: auth cleanup disabled (AUTH_CLEANUP_ENABLED=false)")
	}

	// Approval SLA sweep (Module-3 3.5): remind approvers of hiring-approval steps
	// left pending past their SLA. Disabled by default so a fresh environment never
	// escalates until opted in.
	if cfg.ApprovalSLAEnabled {
		slaTask, err := queue.NewApprovalSLASweepTask(queue.ApprovalSLASweepPayload{})
		if err != nil {
			log.Fatal().Err(err).Msg("build approval sla sweep task failed")
		}
		slaID, err := scheduler.Register(cfg.ApprovalSLACron, slaTask)
		if err != nil {
			log.Fatal().Err(err).Str("cron", cfg.ApprovalSLACron).Msg("register approval sla sweep failed")
		}
		log.Info().Str("cron", cfg.ApprovalSLACron).Str("entry_id", slaID).Msg("scheduler: approval:sla_sweep registered")
	} else {
		log.Info().Msg("scheduler: approval SLA sweep disabled (APPROVAL_SLA_ENABLED=false)")
	}

	if err := scheduler.Run(); err != nil {
		log.Fatal().Err(err).Msg("scheduler error")
	}
}
