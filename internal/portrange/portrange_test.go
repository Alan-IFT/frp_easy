package portrange

import (
	"errors"
	"reflect"
	"testing"
)

// TestParse_TableDriven 覆盖 02 §C.1.5 portrange_test.go 表格 + 边界。
// 至少 11 用例（02 §C.1.5 要求）。
func TestParse_TableDriven(t *testing.T) {
	cases := []struct {
		name     string
		expr     string
		maxCount int
		wantOK   []int
		wantErr  error // sentinel；用 errors.Is 比对
	}{
		{"empty", "", 32, nil, ErrEmpty},
		{"whitespace-only", "   ", 32, nil, ErrEmpty},
		{"single-port", "80", 32, []int{80}, nil},
		{"comma-list", "22,80,443", 32, []int{22, 80, 443}, nil},
		{"range", "6000-6005", 32, []int{6000, 6001, 6002, 6003, 6004, 6005}, nil},
		{"mixed", "6000-6010,7000", 32, []int{6000, 6001, 6002, 6003, 6004, 6005, 6006, 6007, 6008, 6009, 6010, 7000}, nil},
		{"whitespace-tolerance", "  22 , 80 , 443  ", 32, []int{22, 80, 443}, nil},
		{"bad-syntax-alpha", "abc", 32, nil, ErrBadSyntax},
		{"bad-syntax-half-range", "6000-", 32, nil, ErrBadSyntax},
		{"bad-syntax-other-half", "-6000", 32, nil, ErrBadSyntax},
		{"bad-syntax-trailing-comma", "80,", 32, nil, ErrBadSyntax},
		{"bad-syntax-empty-token", "80,,90", 32, nil, ErrBadSyntax},
		{"bad-syntax-double-dash", "6000--6005", 32, nil, ErrPortOutOfRange}, // 解析为 lo=6000, hi=-6005 → 越界优先报
		{"port-too-big", "70000", 32, nil, ErrPortOutOfRange},
		{"port-zero", "0,80", 32, nil, ErrPortOutOfRange},
		{"negative-port", "-1,80", 32, nil, ErrBadSyntax}, // "-1" 落入"半范围"
		{"range-reversed", "6010-6000", 32, nil, ErrRangeReversed},
		{"range-too-many", "6000-7000", 32, nil, nil}, // 1001 → TooManyError 但 errors.Is 不命中标 nil；下面用 errors.As 单独判
		{"duplicate-explicit", "80,80", 32, nil, nil},
		{"duplicate-in-range", "6000-6005,6003", 32, nil, nil},
		{"upper-bound-65535", "65535", 32, []int{65535}, nil},
		{"lower-bound-1", "1", 32, []int{1}, nil},
		{"order-shuffled-sorted-on-output", "443,80,22", 32, []int{22, 80, 443}, nil},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Parse(c.expr, c.maxCount)

			// 特殊用例：TooMany / Duplicate 用 errors.As 而非 errors.Is（带数值）。
			switch c.name {
			case "range-too-many":
				var tm *TooManyError
				if !errors.As(err, &tm) {
					t.Fatalf("expected *TooManyError, got %v", err)
				}
				if tm.Max != 32 || tm.Count <= 32 {
					t.Errorf("TooManyError fields: %+v", tm)
				}
				return
			case "duplicate-explicit":
				var de *DuplicateError
				if !errors.As(err, &de) {
					t.Fatalf("expected *DuplicateError, got %v", err)
				}
				if de.Port != 80 {
					t.Errorf("DuplicateError.Port = %d, want 80", de.Port)
				}
				return
			case "duplicate-in-range":
				var de *DuplicateError
				if !errors.As(err, &de) {
					t.Fatalf("expected *DuplicateError, got %v", err)
				}
				if de.Port != 6003 {
					t.Errorf("DuplicateError.Port = %d, want 6003", de.Port)
				}
				return
			}

			if c.wantErr != nil {
				if !errors.Is(err, c.wantErr) {
					t.Fatalf("err = %v, want errors.Is == %v", err, c.wantErr)
				}
				if got != nil {
					t.Errorf("got = %v, want nil on error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if !reflect.DeepEqual(got, c.wantOK) {
				t.Errorf("got = %v, want %v", got, c.wantOK)
			}
		})
	}
}

// TestParse_NegativeMaxCount 校验防御：maxCount 必须 > 0。
func TestParse_NegativeMaxCount(t *testing.T) {
	_, err := Parse("80", 0)
	if err == nil {
		t.Fatal("expected error on maxCount=0")
	}
	_, err = Parse("80", -1)
	if err == nil {
		t.Fatal("expected error on maxCount=-1")
	}
}

// TestBadSyntaxError_Is 验证 *BadSyntaxError 满足 errors.Is(err, ErrBadSyntax)。
func TestBadSyntaxError_Is(t *testing.T) {
	e := &BadSyntaxError{Token: "xyz"}
	if !errors.Is(e, ErrBadSyntax) {
		t.Error("*BadSyntaxError should satisfy errors.Is(err, ErrBadSyntax)")
	}
}
