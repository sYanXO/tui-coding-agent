package token

import (
	"fmt"
	"sync/atomic"
)

// Counter accumulates token usage across an entire session.
type Counter struct {
	totalIn  atomic.Int64
	totalOut atomic.Int64
}

// Record adds a single-turn observation and returns the formatted per-turn string.
func (c *Counter) Record(in, out int32) string {
	c.totalIn.Add(int64(in))
	c.totalOut.Add(int64(out))
	return fmt.Sprintf(
		"[tokens: %s in / %s out / %s total | session: %s in / %s out / %s total]",
		fmtN(in), fmtN(out), fmtN(in+out),
		fmtN64(c.totalIn.Load()), fmtN64(c.totalOut.Load()),
		fmtN64(c.totalIn.Load()+c.totalOut.Load()),
	)
}

// Summary returns a multi-line session-end summary string.
func (c *Counter) Summary() string {
	in := c.totalIn.Load()
	out := c.totalOut.Load()
	return fmt.Sprintf(
		"Session token usage:\n  Input  : %s\n  Output : %s\n  Total  : %s",
		fmtN64(in), fmtN64(out), fmtN64(in+out),
	)
}

func fmtN(n int32) string { return fmtN64(int64(n)) }

func fmtN64(n int64) string {
	s := fmt.Sprintf("%d", n)
	out := make([]byte, 0, len(s)+(len(s)-1)/3)
	for i, ch := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(ch))
	}
	return string(out)
}
