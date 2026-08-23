package grapheme_test
import ("testing"; "task181-fontproof/internal/grapheme")
func TestBug03_NumericScriptsStaySeparate(t *testing.T){ got:=grapheme.Segmenter("12١٢"); if len(got)!=2 { t.Fatalf("clusters=%d, want 2: %+v",len(got),got) } }
