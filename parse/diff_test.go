package parse

import (
	"math/rand"
	"strconv"
	"testing"
)

// lcsLen is the textbook O(n·m) LCS length, the reference for lcsPairs.
func lcsLen(a, b []string) int {
	L := make([][]int, len(a)+1)
	for i := range L {
		L[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				L[i][j] = L[i+1][j+1] + 1
			} else {
				L[i][j] = max(L[i+1][j], L[i][j+1])
			}
		}
	}
	return L[0][0]
}

// The linear-space diff finds a longest common subsequence: as long as the
// table's, in order, each pair matching equal elements. Small alphabets make
// many equal elements, so position must break ties.
func TestLCSPairsIsALongestCommonSubsequence(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	gen := func(n, k int) []string {
		s := make([]string, n)
		for i := range s {
			s[i] = strconv.Itoa(r.Intn(k))
		}
		return s
	}
	for round := 0; round < 3000; round++ {
		a, b := gen(r.Intn(30), 1+r.Intn(6)), gen(r.Intn(30), 1+r.Intn(6))
		pairs := lcsPairs(a, b)
		if len(pairs) != lcsLen(a, b) {
			t.Fatalf("a=%v b=%v: %d pairs, want %d", a, b, len(pairs), lcsLen(a, b))
		}
		for i, p := range pairs {
			if a[p[0]] != b[p[1]] || (i > 0 && (p[0] <= pairs[i-1][0] || p[1] <= pairs[i-1][1])) {
				t.Fatalf("a=%v b=%v: bad pairs %v", a, b, pairs)
			}
		}
	}
}
