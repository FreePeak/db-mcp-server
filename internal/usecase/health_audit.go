package usecase

import (
	"context"
	"fmt"
	"strings"
)

// Combined health audit: fan the per-setting auditors out in one call
// so an agent gets a full configuration report instead of invoking
// each audit individually and diffing mentally. Engine-unsupported
// checks are omitted silently (their probes fail fast without a DB
// roundtrip); genuine failures stay visible as error sections; the
// summary line counts WARNINGs so severity is visible at a glance.
// Data-scan audits (orphans, PII) are excluded — they are workload
// reports, not configuration checks.

type auditEntry struct {
	name string
	run  func(*DatabaseUseCase, context.Context, string) (string, error)
}

var configAuditors = []auditEntry{
	{"aborted_connections", (*DatabaseUseCase).AuditAbortedConnections},
	{"auto_increment", (*DatabaseUseCase).AuditAutoIncrement},
	{"autovacuum_naptime", (*DatabaseUseCase).AuditAutovacuumNaptime},
	{"av_throttle", (*DatabaseUseCase).AuditAVThrottle},
	{"back_log", (*DatabaseUseCase).AuditBackLog},
	{"binlogs", (*DatabaseUseCase).AuditBinaryLogs},
	{"binlog_format", (*DatabaseUseCase).AuditBinlogFormat},
	{"binlog_row_image", (*DatabaseUseCase).AuditBinlogRowImage},
	{"buffer_pool", (*DatabaseUseCase).AuditBufferPool},
	{"busy_timeout", (*DatabaseUseCase).AuditBusyTimeout},
	{"charsets", (*DatabaseUseCase).AuditCharsets},
	{"checkpoint_timeout", (*DatabaseUseCase).AuditCheckpointTimeout},
	{"crash_safety", (*DatabaseUseCase).AuditCrashSafety},
	{"doublewrite", (*DatabaseUseCase).AuditDoublewrite},
	{"durability", (*DatabaseUseCase).AuditDurability},
	{"effective_cache", (*DatabaseUseCase).AuditEffectiveCache},
	{"effective_io_concurrency", (*DatabaseUseCase).AuditEffectiveIOConcurrency},
	{"fk_enforcement", (*DatabaseUseCase).AuditFKEnforcement},
	{"flush_method", (*DatabaseUseCase).AuditFlushMethod},
	{"flush_neighbors", (*DatabaseUseCase).AuditFlushNeighbors},
	{"io_capacity", (*DatabaseUseCase).AuditIOCapacity},
	{"jit", (*DatabaseUseCase).AuditJIT},
	{"log_buffer", (*DatabaseUseCase).AuditLogBuffer},
	{"log_checkpoints", (*DatabaseUseCase).AuditLogCheckpoints},
	{"log_lock_waits", (*DatabaseUseCase).AuditLogLockWaits},
	{"maintenance_work_mem", (*DatabaseUseCase).AuditMaintenanceWorkMem},
	{"max_allowed_packet", (*DatabaseUseCase).AuditMaxAllowedPacket},
	{"open_files_limit", (*DatabaseUseCase).AuditOpenFilesLimit},
	{"password_auth", (*DatabaseUseCase).AuditPasswordAuth},
	{"random_page_cost", (*DatabaseUseCase).AuditRandomPageCost},
	{"redo_log", (*DatabaseUseCase).AuditRedoLog},
	{"replication_slots", (*DatabaseUseCase).AuditReplicationSlots},
	{"shared_buffers", (*DatabaseUseCase).AuditSharedBuffers},
	{"slot_wal_cap", (*DatabaseUseCase).AuditSlotWalCap},
	{"slow_log", (*DatabaseUseCase).AuditSlowLog},
	{"slow_query_log", (*DatabaseUseCase).AuditSlowQueryLog},
	{"ssl_min_protocol", (*DatabaseUseCase).AuditSSLMinProtocol},
	{"statistics_target", (*DatabaseUseCase).AuditStatisticsTarget},
	{"strict_mode", (*DatabaseUseCase).AuditStrictMode},
	{"sync_binlog", (*DatabaseUseCase).AuditSyncBinlog},
	{"sync_commit", (*DatabaseUseCase).AuditSyncCommit},
	{"table_cache", (*DatabaseUseCase).AuditTableCache},
	{"tcp_keepalives", (*DatabaseUseCase).AuditTCPKeepalives},
	{"temp_file_limit", (*DatabaseUseCase).AuditTempFileLimit},
	{"thread_cache", (*DatabaseUseCase).AuditThreadCache},
	{"timeout_guards", (*DatabaseUseCase).AuditTimeoutGuards},
	{"track_counts", (*DatabaseUseCase).AuditTrackCounts},
	{"track_io_timing", (*DatabaseUseCase).AuditTrackIoTiming},
	{"wal_compression", (*DatabaseUseCase).AuditWalCompression},
	{"wal_level", (*DatabaseUseCase).AuditWALLevel},
	{"wal_mode", (*DatabaseUseCase).AuditWALMode},
	{"wal_senders", (*DatabaseUseCase).AuditWalSenders},
	{"wal_sender_timeout", (*DatabaseUseCase).AuditWalSenderTimeout},
	{"wait_timeout", (*DatabaseUseCase).AuditWaitTimeout},
}

// auditResult is one check's outcome, rendered by renderHealthAudit.
type auditResult struct {
	name string
	out  string
	err  error
}

func isUnsupportedEngineErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not available for engine")
}

// renderHealthAudit composes the report: header summary, then each
// applicable check as its own section.
func renderHealthAudit(dbID, dbType string, results []auditResult) string {
	warnings := 0
	var sections []string
	for _, r := range results {
		if isUnsupportedEngineErr(r.err) {
			continue
		}
		if r.err != nil {
			sections = append(sections,
				fmt.Sprintf("== %s ==\n(error: %v)", r.name, r.err))
			continue
		}
		if strings.Contains(r.out, "WARNING") {
			warnings++
		}
		sections = append(sections, fmt.Sprintf("== %s ==\n%s", r.name, strings.TrimRight(r.out, "\n")))
	}
	return fmt.Sprintf("Health audit for %s (%s): %d check(s), %d warning(s)\n\n%s",
		dbID, dbType, len(results)-countUnsupported(results), warnings,
		strings.Join(sections, "\n\n"))
}

func countUnsupported(results []auditResult) int {
	n := 0
	for _, r := range results {
		if isUnsupportedEngineErr(r.err) {
			n++
		}
	}
	return n
}

// RunHealthAudit runs every configuration auditor against dbID and
// renders one combined report. Individual check failures degrade to
// error sections — the report never fails wholesale.
func (uc *DatabaseUseCase) RunHealthAudit(ctx context.Context, dbID string) (string, error) {
	dbType, err := uc.repo.GetDatabaseType(dbID)
	if err != nil {
		return "", fmt.Errorf("failed to get database type: %w", err)
	}
	results := make([]auditResult, len(configAuditors))
	for i, entry := range configAuditors {
		results[i] = runAuditSafe(entry, uc, ctx, dbID)
	}
	return renderHealthAudit(dbID, dbType, results), nil
}

// runAuditSafe isolates one check: a panic in any single auditor (a
// driver quirk, an unexpected catalog shape) becomes an error section
// instead of killing the whole report.
func runAuditSafe(entry auditEntry, uc *DatabaseUseCase, ctx context.Context, dbID string) (res auditResult) {
	defer func() {
		if r := recover(); r != nil {
			res = auditResult{name: entry.name, err: fmt.Errorf("check panicked: %v", r)}
		}
	}()
	out, err := entry.run(uc, ctx, dbID)
	return auditResult{name: entry.name, out: out, err: err}
}
