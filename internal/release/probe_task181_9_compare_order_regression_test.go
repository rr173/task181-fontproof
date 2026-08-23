package release_test
import ("testing"; "task181-fontproof/internal/model"; "task181-fontproof/internal/release")
func TestBug09_CompareListsStable(t *testing.T){ a:=&release.Snapshot{RuleVersion:1}; b:=&release.Snapshot{RuleVersion:2,Rules:[]release.FrozenRule{{RuleID:"2",Name:"z"},{RuleID:"1",Name:"a"}}}; d:=release.Compare(a,b); if len(d.AddedRules)!=2 || d.AddedRules[0]!="a" || d.AddedRules[1]!="z" { t.Fatalf("added=%v",d.AddedRules) }; _=model.ConfigDiff{} }
