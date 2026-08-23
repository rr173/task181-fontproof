package fallback_test
import ("testing"; "task181-fontproof/internal/fallback")
func TestBug06_CrossRuleCycleDetected(t *testing.T){ if err:=fallback.CheckCycle(map[string][]string{"r1":{"A","B"},"r2":{"B","A"}}); err==nil { t.Fatal("cycle must be detected") } }
