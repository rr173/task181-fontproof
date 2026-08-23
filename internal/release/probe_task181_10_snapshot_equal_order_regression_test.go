package release_test
import ("testing"; "task181-fontproof/internal/release")
func TestBug10_SnapshotEqualIgnoresOrder(t *testing.T){ a:=&release.Snapshot{RuleVersion:1,Rules:[]release.FrozenRule{{RuleID:"a",Name:"a"},{RuleID:"b",Name:"b"}}}; b:=&release.Snapshot{RuleVersion:1,Rules:[]release.FrozenRule{{RuleID:"b",Name:"b"},{RuleID:"a",Name:"a"}}}; if !release.SnapshotEqual(a,b){ t.Fatal("equivalent snapshots with reordered entries should compare equal") } }
