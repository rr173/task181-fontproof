package typeface

import (
	"reflect"
	"testing"

	"task181-fontproof/internal/model"
)

func TestFingerprintStable(t *testing.T) {
	in := model.FontInput{
		Name:        "Noto Sans Latin",
		Family:      "Noto Sans",
		SpecVersion: "1.0",
		Ranges:      []model.Range{{Start: 0x0020, End: 0x007E}, {Start: 0x00C0, End: 0x024F}},
		Features:    []string{"ccmp", "mark", "locl"},
		Scripts:     []string{"Latn", "Zyyy"},
	}
	fp1 := Fingerprint(in)
	fp2 := Fingerprint(in)
	if fp1 != fp2 {
		t.Fatalf("fingerprint not stable for identical input: %q vs %q", fp1, fp2)
	}
	// 顺序无关：乱序的 Features/Scripts 应得到相同指纹。
	shuffled := in
	shuffled.Features = []string{"mark", "locl", "ccmp"}
	shuffled.Scripts = []string{"Zyyy", "Latn"}
	if Fingerprint(shuffled) != fp1 {
		t.Fatal("fingerprint should be order-independent for features/scripts")
	}
}

// TestFingerprintDoesNotMutateInput 验证调用 Fingerprint 后调用方传入的
// Features/Scripts/Ranges 配置保持原样（修复此前副作用：指纹计算就地排序改写入参）。
func TestFingerprintDoesNotMutateInput(t *testing.T) {
	features := []string{"ccmp", "mark", "locl"}
	scripts := []string{"Latn", "Zyyy"}
	ranges := []model.Range{{Start: 0x00C0, End: 0x024F}, {Start: 0x0020, End: 0x007E}}
	in := model.FontInput{
		Name:        "Noto Sans Latin",
		Family:      "Noto Sans",
		SpecVersion: "1.0",
		Ranges:      ranges,
		Features:    features,
		Scripts:     scripts,
	}
	wantFeatures := append([]string(nil), features...)
	wantScripts := append([]string(nil), scripts...)
	wantRanges := append([]model.Range(nil), ranges...)

	_ = Fingerprint(in)

	if !reflect.DeepEqual(in.Features, wantFeatures) {
		t.Fatalf("Fingerprint mutated Features: got %v, want %v", in.Features, wantFeatures)
	}
	if !reflect.DeepEqual(in.Scripts, wantScripts) {
		t.Fatalf("Fingerprint mutated Scripts: got %v, want %v", in.Scripts, wantScripts)
	}
	if !reflect.DeepEqual(in.Ranges, wantRanges) {
		t.Fatalf("Fingerprint mutated Ranges: got %v, want %v", in.Ranges, wantRanges)
	}
}
