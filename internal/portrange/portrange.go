// Package portrange 解析端口表达式。
//
// 表达式语法（02 §C.1.2）：
//
//	token := <int>                    // 单端口
//	       | <int> "-" <int>          // 闭区间，左 ≤ 右
//	expr := token ("," token)*        // 逗号分隔，空格被 trim
//
// 语义：
//   - 端口范围 ∈ [1, 65535]。
//   - 单次最多 maxCount 个端口（由调用方传入，T-018 上限 32）。
//   - 展开后**去重**：相同端口出现 ≥2 次 → 返回 ErrDuplicate 并附端口值。
//   - 不允许左 > 右。
//   - 不引入新依赖（仅 stdlib）。
//
// 错误使用 sentinel + (可选附加值) 方式：调用方用 errors.Is 分流，用 errors.As 取附加值。
package portrange

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Sentinel errors —— 调用方用 errors.Is 分流。
var (
	ErrEmpty          = errors.New("portrange: 端口表达式必填")
	ErrBadSyntax      = errors.New("portrange: 端口表达式语法错误")
	ErrPortOutOfRange = errors.New("portrange: 端口必须在 1-65535 之间")
	ErrRangeReversed  = errors.New("portrange: 端口范围左端必须 ≤ 右端")
)

// DuplicateError 表示展开后含重复端口。
type DuplicateError struct {
	Port int
}

func (e *DuplicateError) Error() string {
	return fmt.Sprintf("portrange: 端口表达式含重复项：%d", e.Port)
}

// TooManyError 表示展开后总数超过 maxCount。
type TooManyError struct {
	Count int
	Max   int
}

func (e *TooManyError) Error() string {
	return fmt.Sprintf("portrange: 单次端口数超过 %d 上限（当前 %d）", e.Max, e.Count)
}

// BadSyntaxError 表示语法错误，附问题 token 便于用户调试。
type BadSyntaxError struct {
	Token string
}

func (e *BadSyntaxError) Error() string {
	if e.Token == "" {
		return ErrBadSyntax.Error()
	}
	return fmt.Sprintf("portrange: 端口表达式语法错误（token=%q）", e.Token)
}

// Is 让 BadSyntaxError 满足 errors.Is(err, ErrBadSyntax)。
func (e *BadSyntaxError) Is(target error) bool { return target == ErrBadSyntax }

// Parse 解析端口表达式 expr，返回去重后**升序**端口数组。
// maxCount 必须 > 0；展开后端口数 > maxCount → TooManyError（也匹配 errors.Is(err, ErrBadSyntax)
// 由 Is 显式拒绝，仅 TooMany 单独类型 —— 调用方用 errors.As 取详细数字）。
//
// 实现注意：先解析 + 累加到 set 再排序，单次扫描；不对 Pre-allocate 强求。
func Parse(expr string, maxCount int) ([]int, error) {
	if maxCount <= 0 {
		return nil, fmt.Errorf("portrange.Parse: maxCount 必须 > 0")
	}
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, ErrEmpty
	}

	seen := make(map[int]bool)
	for _, raw := range strings.Split(expr, ",") {
		tok := strings.TrimSpace(raw)
		if tok == "" {
			return nil, &BadSyntaxError{Token: raw}
		}

		// 范围 token：含一个 '-'。
		if i := strings.IndexByte(tok, '-'); i >= 0 {
			loStr := strings.TrimSpace(tok[:i])
			hiStr := strings.TrimSpace(tok[i+1:])
			if loStr == "" || hiStr == "" {
				return nil, &BadSyntaxError{Token: tok}
			}
			lo, err1 := strconv.Atoi(loStr)
			hi, err2 := strconv.Atoi(hiStr)
			if err1 != nil || err2 != nil {
				return nil, &BadSyntaxError{Token: tok}
			}
			if lo < 1 || lo > 65535 || hi < 1 || hi > 65535 {
				return nil, ErrPortOutOfRange
			}
			if lo > hi {
				return nil, ErrRangeReversed
			}
			for p := lo; p <= hi; p++ {
				if seen[p] {
					return nil, &DuplicateError{Port: p}
				}
				seen[p] = true
				if len(seen) > maxCount {
					return nil, &TooManyError{Count: len(seen), Max: maxCount}
				}
			}
			continue
		}

		// 单端口 token。
		p, err := strconv.Atoi(tok)
		if err != nil {
			return nil, &BadSyntaxError{Token: tok}
		}
		if p < 1 || p > 65535 {
			return nil, ErrPortOutOfRange
		}
		if seen[p] {
			return nil, &DuplicateError{Port: p}
		}
		seen[p] = true
		if len(seen) > maxCount {
			return nil, &TooManyError{Count: len(seen), Max: maxCount}
		}
	}

	out := make([]int, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Ints(out)
	return out, nil
}
