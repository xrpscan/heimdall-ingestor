package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildValueString(t *testing.T) {
	tests := []struct {
		name     string
		rows     int
		cols     int
		expected string
	}{
		{
			name:     "single row single column",
			rows:     1,
			cols:     1,
			expected: "($1)",
		},
		{
			name:     "single row multiple columns",
			rows:     1,
			cols:     3,
			expected: "($1, $2, $3)",
		},
		{
			name:     "multiple rows single column",
			rows:     3,
			cols:     1,
			expected: "($1), ($2), ($3)",
		},
		{
			name:     "multiple rows multiple columns",
			rows:     2,
			cols:     3,
			expected: "($1, $2, $3), ($4, $5, $6)",
		},
		{
			name:     "large row and column count",
			rows:     3,
			cols:     5,
			expected: "($1, $2, $3, $4, $5), ($6, $7, $8, $9, $10), ($11, $12, $13, $14, $15)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildValueString(tt.rows, tt.cols)
			require.Equal(t, tt.expected, result)
		})
	}
}
