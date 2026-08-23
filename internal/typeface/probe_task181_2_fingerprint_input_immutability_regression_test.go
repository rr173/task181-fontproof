package typeface_test
import ("testing"; "task181-fontproof/internal/model"; "task181-fontproof/internal/typeface")
func TestBug02_FingerprintDoesNotMutateInput(t *testing.T){ in:=model.FontInput{Name:"N",Family:"F",Features:[]string{"z","a"},Scripts:[]string{"Zyyy","Latn"}}; _=typeface.Fingerprint(in); if in.Features[0]!="z" || in.Scripts[0]!="Zyyy" { t.Fatalf("fingerprint mutated input: %+v",in) } }
