package store

import (
	"fmt"
	"strings"
)

// buildValueString constructs the VALUES placeholder string for a parameterized
// bulk INSERT query. For example, buildValueString(2, 3) returns
// "($1, $2, $3), ($4, $5, $6)".
func buildValueString(rowCount, colCount int) string {
	// Create a single row string. It looks like: ($%d, $%d, $%d, ...)
	singleRow := strings.Repeat("$%d, ", colCount)
	singleRow = strings.TrimSuffix(singleRow, ", ")
	singleRow = "(" + singleRow + "), "

	// Array of numbers to convert
	// `($%d, $%d, $%d), ($%d, $%d, $%d),($%d, $%d, $%d)` to
	// `($1, $2, $3), ($4, $5, $6), ($7, $8, $9)`
	// ^^ This example is for a 3 column query but may extend to any number of columns.
	numArgs := make([]any, rowCount*colCount)
	for i := range numArgs {
		numArgs[i] = i + 1
	}

	// Repeat the singleRow string for all rows.
	valueString := strings.Repeat(singleRow, rowCount)
	// Replace the %d's with numbers.
	valueString = fmt.Sprintf(valueString, numArgs...)
	// Remove trailing comma-space from the earlier string-building and that's it.
	return strings.TrimSuffix(valueString, ", ")
}
