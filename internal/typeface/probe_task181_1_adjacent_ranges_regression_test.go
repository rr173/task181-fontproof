package typeface_test
import ("testing"; "task181-fontproof/internal/model"; "task181-fontproof/internal/typeface")
func TestBug01_AdjacentRangesMerge(t *testing.T){ got:=typeface.NormalizeRanges([]model.Range{{Start:1,End:2},{Start:3,End:4}}); if len(got)!=1 || got[0].Start!=1 || got[0].End!=4 { t.Fatalf("ranges=%v, want one merged range",got) } }
