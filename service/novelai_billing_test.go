package service

import (
	"testing"
)

func TestClassifyNovelAIRequest(t *testing.T) {
	tests := []struct {
		name              string
		spec              NovelAIParamSpec
		expectedType      NovelAIBillingType
		expectedPowerCost int
		expectedAnlasCost int
	}{
		{
			name: "nai-diffusion-3 代表免费图 (0 扣除)",
			spec: NovelAIParamSpec{
				Model:  "nai-diffusion-3",
				Width:  832,
				Height: 1216,
				Steps:  28,
				N:      1,
				Action: "generate",
			},
			expectedType:      BillingTypeFree,
			expectedPowerCost: 0,
			expectedAnlasCost: 0,
		},
		{
			name: "nai-diffusion-4-full 代表 5 Anlas 图 (单张)",
			spec: NovelAIParamSpec{
				Model:  "nai-diffusion-4-full",
				Width:  832,
				Height: 1216,
				Steps:  28,
				N:      1,
			},
			expectedType:      BillingTypeAnlas,
			expectedPowerCost: 0,
			expectedAnlasCost: 5,
		},
		{
			name: "nai-diffusion-4-full 代表 5 Anlas 图 (N=2 批量)",
			spec: NovelAIParamSpec{
				Model:  "nai-diffusion-4-full",
				Width:  832,
				Height: 1216,
				Steps:  28,
				N:      2,
			},
			expectedType:      BillingTypeAnlas,
			expectedPowerCost: 0,
			expectedAnlasCost: 10,
		},
		{
			name: "nai-diffusion-4-5-full 代表 1 电量图 (单张)",
			spec: NovelAIParamSpec{
				Model:  "nai-diffusion-4-5-full",
				Width:  832,
				Height: 1216,
				Steps:  28,
				N:      1,
			},
			expectedType:      BillingTypePower,
			expectedPowerCost: 1,
			expectedAnlasCost: 0,
		},
		{
			name: "nai-diffusion-4-5-full 代表 1 电量图 (N=3 批量)",
			spec: NovelAIParamSpec{
				Model:  "nai-diffusion-4-5-full",
				Width:  832,
				Height: 1216,
				Steps:  28,
				N:      3,
			},
			expectedType:      BillingTypePower,
			expectedPowerCost: 3,
			expectedAnlasCost: 0,
		},
		{
			name: "nai-diffusion-5 代表 1 电量 + 5 Anlas 图 (单张)",
			spec: NovelAIParamSpec{
				Model:  "nai-diffusion-5",
				Width:  832,
				Height: 1216,
				Steps:  28,
				N:      1,
			},
			expectedType:      BillingTypeComposite,
			expectedPowerCost: 1,
			expectedAnlasCost: 5,
		},
		{
			name: "nai-diffusion-5 代表 1 电量 + 5 Anlas 图 (N=2 批量)",
			spec: NovelAIParamSpec{
				Model:  "nai-diffusion-5",
				Width:  832,
				Height: 1216,
				Steps:  28,
				N:      2,
			},
			expectedType:      BillingTypeComposite,
			expectedPowerCost: 2,
			expectedAnlasCost: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := ClassifyNovelAIRequest(tt.spec)
			if res.Type != tt.expectedType {
				t.Errorf("expected type %v, got %v", tt.expectedType, res.Type)
			}
			if res.PowerCost != tt.expectedPowerCost {
				t.Errorf("expected power cost %v, got %v", tt.expectedPowerCost, res.PowerCost)
			}
			if res.AnlasCost != tt.expectedAnlasCost {
				t.Errorf("expected anlas cost %v, got %v", tt.expectedAnlasCost, res.AnlasCost)
			}
		})
	}
}

func TestBillingTypeString(t *testing.T) {
	if BillingTypeFree.String() != "free" {
		t.Errorf("expected free, got %s", BillingTypeFree.String())
	}
	if BillingTypePower.String() != "power" {
		t.Errorf("expected power, got %s", BillingTypePower.String())
	}
	if BillingTypeAnlas.String() != "anlas" {
		t.Errorf("expected anlas, got %s", BillingTypeAnlas.String())
	}
	if BillingTypeComposite.String() != "composite" {
		t.Errorf("expected composite, got %s", BillingTypeComposite.String())
	}
	if NovelAIBillingType(99).String() != "unknown" {
		t.Errorf("expected unknown, got %s", NovelAIBillingType(99).String())
	}
}

func TestPreCheckFreeAllowed(t *testing.T) {
	// Free requests must pass immediately with nil without querying DB
	err := PreCheckNovelAIBilling(0, NovelAIBillingResult{Type: BillingTypeFree})
	if err != nil {
		t.Errorf("expected nil for free billing precheck, got %v", err)
	}
}
