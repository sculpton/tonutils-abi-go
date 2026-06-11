package main

import (
	"regexp"
	"strings"
	"testing"
)

func TestGenerateNestedFallibleStackParams(t *testing.T) {
	one := 1
	zero := 0
	wideTypeID := 71
	wideWidth := 2
	unionPayloadID := 11
	unionNullID := 12
	unionWidth := 2
	payload := abiType{Kind: "cellOf", Inner: &abiType{Kind: "StructRef", StructName: "Point"}}
	maybePayload := abiType{Kind: "nullable", Inner: &payload}
	widePayload := abiType{
		Kind:        "nullable",
		Inner:       &payload,
		StackTypeID: &wideTypeID,
		StackWidth:  &wideWidth,
	}
	unionPayload := abiType{
		Kind:       "union",
		Items:      []abiType{payload, {Kind: "null"}},
		StackWidth: &unionWidth,
		Variants: []abiTypeVariant{
			{StackTypeID: &unionPayloadID, StackWidth: &one},
			{StackTypeID: &unionNullID, StackWidth: &zero},
		},
	}

	abi := abiFile{
		ContractName: "Sample",
		Declarations: []declaration{
			{
				Kind: "struct",
				Name: "Point",
				Fields: []field{
					{Name: "x", Type: abiType{Kind: "uintN", N: 32}},
				},
			},
			{
				Kind: "struct",
				Name: "Envelope",
				Fields: []field{
					{Name: "payload", Type: payload},
				},
			},
		},
		GetMethods: []getMethod{
			{
				Name: "store",
				Parameters: []parameter{
					{Name: "cells", Type: abiType{Kind: "arrayOf", Inner: &payload}},
					{Name: "list", Type: abiType{Kind: "lispListOf", Inner: &payload}},
					{Name: "maybe", Type: maybePayload},
					{Name: "wide", Type: widePayload},
					{Name: "envelope", Type: abiType{Kind: "StructRef", StructName: "Envelope"}},
					{Name: "choice", Type: unionPayload},
				},
				ReturnType: abiType{Kind: "void"},
			},
		},
	}

	result, err := newGenerator([]abiFile{abi}, "sample").Generate()
	src := result.Source
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	text := string(src)
	if strings.Contains(text, "nested fallible stack encoding") {
		t.Fatalf("generated source still uses the old nested fallible fallback:\n%s", src)
	}
	if strings.Contains(text, "mustStackCellOf") {
		t.Fatalf("generated fallible stack path should not emit panic-based cell encoders:\n%s", src)
	}
	if output, err := compileGeneratedWrapper(t, src); err != nil {
		t.Fatalf("generated wrapper does not compile: %v\n%s", err, output)
	}

	for _, want := range []string{
		`cellsStack, err := stackArrayErr\(cells, func\(v Point\) \(any, error\) \{ return stackCellOf\(v\) \}\)`,
		`listStack, err := stackLispListErr\(list, func\(v Point\) \(any, error\) \{ return stackCellOf\(v\) \}\)`,
		`maybeStack, err := stackNullablePtrErr\(maybe, func\(v Point\) \(any, error\) \{ return stackCellOf\(v\) \}\)`,
		`wideStack, err := stackWideNullablePtrErr\(wide, 2, int64\(71\), func\(v Point\) \(\[\]any, error\) \{ return stackSingleValue\(stackCellOf\(v\)\) \}\)`,
		`envelopeStack, err := stackEnvelopeErr\(envelope\)`,
		`choiceStack, err := stackUnionErr\(choice\)`,
		`func stackEnvelopeErr\(value \*Envelope\) \(\[\]any, error\)`,
		`envelopePayloadStack, err := stackCellOf\(value\.Payload\)`,
		`func stackUnionErr\(union \*Union\) \(\[\]any, error\)`,
	} {
		if !regexp.MustCompile(want).Match(src) {
			t.Fatalf("generated source does not match %q:\n%s", want, src)
		}
	}
}
