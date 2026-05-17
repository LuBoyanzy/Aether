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
