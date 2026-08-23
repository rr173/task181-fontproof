package typeface

import (
	"testing"

	"task181-fontproof/internal/model"
)

func TestNormalizeRangesMergesAdjacent(t *testing.T) {
	// 相邻区间 007F→0080 应合并为单一连续区间，而非被当作断开的两段。
	// 这是导致覆盖证明出现多余断点的根因。
	rs := []model.Range{
		{Start: 0x0041, End: 0x007F}, // A..Å
		{Start: 0x0080, End: 0x00FF}, // 相邻：下一区间起点恰为上一区间终点 +1
	}
	got := NormalizeRanges(rs)
	if len(got) != 1 {
		t.Fatalf("adjacent ranges should merge into one, got %d: %v", len(got), got)
	}
	if got[0].Start != 0x0041 || got[0].End != 0x00FF {
		t.Fatalf("merged range should span 0041-00FF, got %04X-%04X", got[0].Start, got[0].End)
	}
}

func TestNormalizeRangesPreservesGap(t *testing.T) {
	// 真正存在间隔的边界（终点 +2 才到下一区间起点）必须保留为独立区间。
	rs := []model.Range{
		{Start: 0x0041, End: 0x007F},
		{Start: 0x0081, End: 0x00FF}, // 0080 缺失 → 真正的间隔
	}
	got := NormalizeRanges(rs)
	if len(got) != 2 {
		t.Fatalf("gapped ranges must stay separate, got %d: %v", len(got), got)
	}
	if got[0].End != 0x007F || got[1].Start != 0x0081 {
		t.Fatalf("gap boundary not preserved: %v", got)
	}
}

func TestNormalizeRangesMergesOverlap(t *testing.T) {
	rs := []model.Range{
		{Start: 0x0041, End: 0x00FF},
		{Start: 0x00C0, End: 0x024F}, // 重叠
	}
	got := NormalizeRanges(rs)
	if len(got) != 1 || got[0].Start != 0x0041 || got[0].End != 0x024F {
		t.Fatalf("overlapping ranges should merge: %v", got)
	}
}

func TestNormalizeRangesHandlesContiguousAndUnsorted(t *testing.T) {
	// 未排序 + 相邻 + 重叠混合：全部相邻/重叠段应归并为单一连续段。
	rs := []model.Range{
		{Start: 0x0030, End: 0x0032}, // 0-2
		{Start: 0x0033, End: 0x0039}, // 3-9 相邻
		{Start: 0x0041, End: 0x007A}, // A-z（与上面有间隔，保留）
	}
	got := NormalizeRanges(rs)
	if len(got) != 2 {
		t.Fatalf("expected 2 ranges after merge, got %d: %v", len(got), got)
	}
	if got[0].Start != 0x0030 || got[0].End != 0x0039 {
		t.Fatalf("digits should merge to 0030-0039, got %04X-%04X", got[0].Start, got[0].End)
	}
	if got[1].Start != 0x0041 || got[1].End != 0x007A {
		t.Fatalf("letters range not preserved: %04X-%04X", got[1].Start, got[1].End)
	}
}

func TestNormalizeRangesMaxCodepointAdjacent(t *testing.T) {
	// 边界：相邻于 Unicode 上限 0x10FFFF，last.End+1 不应溢出（rune 为 int32）。
	rs := []model.Range{
		{Start: 0x10FFFE, End: 0x10FFFE},
		{Start: 0x10FFFF, End: 0x10FFFF}, // 相邻
	}
	got := NormalizeRanges(rs)
	if len(got) != 1 || got[0].Start != 0x10FFFE || got[0].End != 0x10FFFF {
		t.Fatalf("adjacent at upper bound should merge without overflow: %v", got)
	}
}
