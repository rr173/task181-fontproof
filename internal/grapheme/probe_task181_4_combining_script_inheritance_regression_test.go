package grapheme_test
import ("testing"; "task181-fontproof/internal/grapheme")
func TestBug04_CombiningMarkInheritsBaseScript(t *testing.T){ got:=grapheme.DetectScript([]rune{'a','\u0301'}); if got!="Latn" { t.Fatalf("script=%s, want Latn",got) } }
