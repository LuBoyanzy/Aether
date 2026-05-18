package hub

import "testing"

func TestMergeIngestMonitorRecordsSortsAndLimits(t *testing.T) {
	records := mergeIngestMonitorRecords(2,
		[]ingestMonitorRecordDTO{
			{ItemCode: "A001", UpdateTime: "2026-05-16T08:00:00Z", CreateTime: "2026-05-16T07:00:00Z"},
			{ItemCode: "A003", UpdateTime: "2026-05-16T09:00:00Z", CreateTime: "2026-05-16T07:00:00Z"},
		},
		[]ingestMonitorRecordDTO{
			{ItemCode: "A002", UpdateTime: "2026-05-16T10:00:00Z", CreateTime: "2026-05-16T07:00:00Z"},
		},
	)

	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0].ItemCode != "A002" || records[1].ItemCode != "A003" {
		t.Fatalf("unexpected record order: %#v", records)
	}
}

func TestBuildIngestMonitorSummaryResponseKeepsFormalAndTrackingSeparate(t *testing.T) {
	scope := ingestMonitorScopeDTO{
		Tenant:     "guochuang",
		RecordType: ingestMonitorFormalRecordType,
	}
	formalSummary := ingestMonitorSummaryCountsDTO{Total: 3, Success: 2, Failure: 1}
	trackingSummary := ingestMonitorSummaryCountsDTO{Total: 5, Failure: 3, Processing: 2}
	formalRecent := []ingestMonitorRecordDTO{
		{ItemCode: "FORMAL_OK", RecordSource: ingestMonitorFormalRecordType, Status: ingestMonitorStatusSuccess},
		{ItemCode: "FORMAL_FAIL", RecordSource: ingestMonitorFormalRecordType, Status: ingestMonitorStatusFailure},
	}
	trackingRecent := []ingestMonitorRecordDTO{
		{ItemCode: "TEMP_UPLOAD_001", RecordSource: ingestMonitorBatchRecordType, Status: ingestMonitorStatusProcessing},
	}
	formalFailures := []ingestMonitorRecordDTO{
		{ItemCode: "FORMAL_FAIL", RecordSource: ingestMonitorFormalRecordType, Status: ingestMonitorStatusFailure},
	}
	trackingFailures := []ingestMonitorRecordDTO{
		{ItemCode: "TEMP_UPLOAD_FAILED", RecordSource: ingestMonitorBatchRecordType, Status: ingestMonitorStatusFailure},
	}

	response := buildIngestMonitorSummaryResponse(
		scope,
		formalSummary,
		trackingSummary,
		formalRecent,
		trackingRecent,
		formalFailures,
		trackingFailures,
	)

	if response.Summary != formalSummary {
		t.Fatalf("formal summary should not include tracking records: %#v", response.Summary)
	}
	if response.TrackingSummary != trackingSummary {
		t.Fatalf("tracking summary should be exposed separately: %#v", response.TrackingSummary)
	}
	if len(response.Recent) != 2 {
		t.Fatalf("formal recent should only contain formal records, got %d", len(response.Recent))
	}
	for _, record := range response.Recent {
		if record.RecordSource != ingestMonitorFormalRecordType {
			t.Fatalf("formal recent contains tracking record: %#v", record)
		}
	}
	if len(response.Failures) != 1 || response.Failures[0].ItemCode != "FORMAL_FAIL" {
		t.Fatalf("formal failures should only contain formal failures: %#v", response.Failures)
	}
	if len(response.TrackingRecent) != 1 || response.TrackingRecent[0].ItemCode != "TEMP_UPLOAD_001" {
		t.Fatalf("tracking recent missing: %#v", response.TrackingRecent)
	}
	if len(response.TrackingFailures) != 1 || response.TrackingFailures[0].ItemCode != "TEMP_UPLOAD_FAILED" {
		t.Fatalf("tracking failures missing: %#v", response.TrackingFailures)
	}
}

func TestBuildIngestMonitorSummaryResponseUsesEmptySlicesForEmptyLists(t *testing.T) {
	response := buildIngestMonitorSummaryResponse(
		ingestMonitorScopeDTO{Tenant: "guochuang", RecordType: ingestMonitorFormalRecordType},
		ingestMonitorSummaryCountsDTO{},
		ingestMonitorSummaryCountsDTO{},
		nil,
		nil,
		nil,
		nil,
	)

	if response.Recent == nil {
		t.Fatal("recent must be an empty slice, not nil")
	}
	if response.Failures == nil {
		t.Fatal("failures must be an empty slice, not nil")
	}
	if response.TrackingRecent == nil {
		t.Fatal("tracking recent must be an empty slice, not nil")
	}
	if response.TrackingFailures == nil {
		t.Fatal("tracking failures must be an empty slice, not nil")
	}
}
