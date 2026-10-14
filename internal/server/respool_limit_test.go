package server

import (
	"fmt"
	"testing"

	"resource-center/internal/respool"
)

func TestCapThreadPageToFirst50(t *testing.T) {
	threads := make([]respool.Thread, 20)
	for i := range threads {
		threads[i].ID = fmt.Sprint(i + 1)
	}

	tests := []struct {
		page      int
		wantCount int
	}{
		{page: 1, wantCount: 20},
		{page: 2, wantCount: 20},
		{page: 3, wantCount: 10},
		{page: 4, wantCount: 0},
	}
	for _, tt := range tests {
		got := capThreadPage(respool.ThreadPage{
			Threads: append([]respool.Thread(nil), threads...),
			Page:    tt.page, PageSize: 20, Total: 7174, TotalPages: 359,
		}, 50)
		if len(got.Threads) != tt.wantCount || got.Total != 50 || got.TotalPages != 3 {
			t.Fatalf("page %d: count=%d total=%d pages=%d", tt.page, len(got.Threads), got.Total, got.TotalPages)
		}
	}
}
