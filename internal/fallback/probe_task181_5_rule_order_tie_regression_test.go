package fallback_test
import ("testing"; "task181-fontproof/internal/fallback"; "task181-fontproof/internal/model")
func TestBug05_RuleOrderTieUsesID(t *testing.T){ got:=fallback.ResolveOrder([]model.FallbackRule{{ID:"b",Name:"same",Priority:1},{ID:"a",Name:"same",Priority:1}}); if len(got)!=2 || got[0]!="a" { t.Fatalf("order=%v, want [a b]",got) } }
