package release

import "testing"

// TestSnapshotCompareIdenticalContentReorderedSlices 验证：内容完全相同
// 但 Rules 切片排列顺序不同的两个快照应判为相等（Same=true），重排不误报。
func TestSnapshotCompareIdenticalContentReorderedSlices(t *testing.T) {
	r1 := FrozenRule{RuleID: "r1", Name: "latin", Priority: 1, FontIDs: []string{"f1"}, Scripts: []string{"Latn"}}
	r2 := FrozenRule{RuleID: "r2", Name: "arabic", Priority: 2, FontIDs: []string{"f2"}, Scripts: []string{"Arab"}}
	font := FrozenFont{FontID: "f1", Name: "F1", Status: "available", Ranges: "0x20-0x7E", Fingerprint: "x"}

	// 两条规则 priority 相同：触发 map 迭代非确定性场景
	r1b := FrozenRule{RuleID: "r1", Name: "latin", Priority: 1, FontIDs: []string{"f1"}, Scripts: []string{"Latn"}}
	r2b := FrozenRule{RuleID: "r2", Name: "arabic", Priority: 1, FontIDs: []string{"f2"}, Scripts: []string{"Arab"}}

	cases := []struct {
		name   string
		rulesA []FrozenRule
		rulesB []FrozenRule
	}{
		{"distinct-priority", []FrozenRule{r1, r2}, []FrozenRule{r2, r1}},
		{"same-priority", []FrozenRule{r1b, r2b}, []FrozenRule{r2b, r1b}},
	}
	for _, c := range cases {
		base := &Snapshot{RuleVersion: 1, Rules: c.rulesA, Fonts: []FrozenFont{font}}
		target := &Snapshot{RuleVersion: 1, Rules: c.rulesB, Fonts: []FrozenFont{font}}
		// 多次运行捕获 map 迭代非确定性
		for i := 0; i < 200; i++ {
			diff := Compare(base, target)
			if !diff.Same || diff.Reordered {
				t.Fatalf("%s iter %d: same-content reordered slices must be equal, got same=%v reordered=%v (%s)",
					c.name, i, diff.Same, diff.Reordered, diff.Summary)
			}
		}
	}
}

// TestSnapshotCompareReorderedPriority 验证：规则集合相同但优先级真正重排时
// 标记 Reordered=true 且 Same=false。
func TestSnapshotCompareReorderedPriority(t *testing.T) {
	r1 := FrozenRule{RuleID: "r1", Name: "latin", Priority: 1, FontIDs: []string{"f1"}, Scripts: []string{"Latn"}}
	r2 := FrozenRule{RuleID: "r2", Name: "arabic", Priority: 2, FontIDs: []string{"f2"}, Scripts: []string{"Arab"}}
	font := FrozenFont{FontID: "f1", Name: "F1", Status: "available", Ranges: "0x20-0x7E", Fingerprint: "x"}

	// 优先级互换：真正重排
	base := &Snapshot{RuleVersion: 1, Rules: []FrozenRule{r1, r2}, Fonts: []FrozenFont{font}}
	target := &Snapshot{RuleVersion: 1, Rules: []FrozenRule{
		{RuleID: "r1", Name: "latin", Priority: 2, FontIDs: []string{"f1"}, Scripts: []string{"Latn"}},
		{RuleID: "r2", Name: "arabic", Priority: 1, FontIDs: []string{"f2"}, Scripts: []string{"Arab"}},
	}, Fonts: []FrozenFont{font}}

	diff := Compare(base, target)
	if !diff.Reordered {
		t.Fatalf("real priority swap should set reordered=true, got %+v", diff)
	}
	if diff.Same {
		t.Fatalf("reordered config should not be same")
	}
}

// TestSnapshotEqualIdenticalReordered 验证 SnapshotEqual 的集合语义：
// 内容相同但顺序不同判等，内容不同判不等。
func TestSnapshotEqualIdenticalReordered(t *testing.T) {
	r1 := FrozenRule{RuleID: "r1", Name: "latin", Priority: 1, FontIDs: []string{"f1", "f2"}, Scripts: []string{"Latn"}}
	r2 := FrozenRule{RuleID: "r2", Name: "arabic", Priority: 2, FontIDs: []string{"f3"}, Scripts: []string{"Arab"}}
	f1 := FrozenFont{FontID: "f1", Name: "F1", Status: "available", Ranges: "0x20-0x7E", Fingerprint: "x"}
	f2 := FrozenFont{FontID: "f2", Name: "F2", Status: "available", Ranges: "0xA0-0xFF", Fingerprint: "y"}

	base := &Snapshot{RuleVersion: 1, Rules: []FrozenRule{r1, r2}, Fonts: []FrozenFont{f1, f2}}
	// 规则、字体顺序均打乱；规则内 FontIDs 顺序也打乱
	target := &Snapshot{RuleVersion: 1, Rules: []FrozenRule{
		{RuleID: "r2", Name: "arabic", Priority: 2, FontIDs: []string{"f3"}, Scripts: []string{"Arab"}},
		{RuleID: "r1", Name: "latin", Priority: 1, FontIDs: []string{"f2", "f1"}, Scripts: []string{"Latn"}},
	}, Fonts: []FrozenFont{f2, f1}}

	if !SnapshotEqual(base, target) {
		t.Fatal("same-content snapshots with reordered slices must be equal")
	}

	// 内容不同：priority 变化
	changed := &Snapshot{RuleVersion: 1, Rules: []FrozenRule{
		{RuleID: "r1", Name: "latin", Priority: 9, FontIDs: []string{"f1"}, Scripts: []string{"Latn"}},
		r2,
	}, Fonts: []FrozenFont{f1, f2}}
	if SnapshotEqual(base, changed) {
		t.Fatal("snapshots differing in rule content must not be equal")
	}
}
