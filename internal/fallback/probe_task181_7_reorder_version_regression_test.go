package fallback_test
import ("testing"; "task181-fontproof/internal/fallback"; "task181-fontproof/internal/model")
func TestBug07_ReorderIncrementsVersion(t *testing.T){ rules:=[]model.FallbackRule{{ID:"r1",Name:"r",Priority:1,Version:3}}; got,err:=fallback.ApplyReorder(rules,map[string][]string{"r1":{"f1"}},[]string{"r1"},""); if err!=nil { t.Fatal(err) }; if got.Version!=4 { t.Fatalf("version=%d, want 4",got.Version) } }
