// Package main demonstrates a complete CSV processing pipeline using byte streams.
// It shows how to use FromReader and ToWriter with CSV encode/decode transformers
// to process CSV data from any io.Reader to any io.Writer.
package main

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/combine"
	"github.com/lguimbarda/min-flow/flow/csv"
	"github.com/lguimbarda/min-flow/flow/filter"
	"github.com/lguimbarda/min-flow/flow/io"
)

func main() {
	fmt.Println("=== CSV Pipeline Examples ===")
	fmt.Println()

	basicPipeline()
	transformPipeline()
	filterAndAggregate()
}

// basicPipeline demonstrates a simple FromReader -> DecodeCSV -> EncodeCSV -> ToWriter pipeline
func basicPipeline() {
	fmt.Println("--- Basic CSV Pipeline ---")
	ctx := context.Background()

	// Input CSV data (simulating reading from a file or network)
	inputCSV := `name,age,city
Alice,30,New York
Bob,25,Los Angeles
Charlie,35,Chicago
`

	// Create a reader from the input string
	reader := strings.NewReader(inputCSV)

	// Create an output buffer (simulating writing to a file or network)
	var output bytes.Buffer

	// Build the pipeline:
	// 1. Read bytes from input
	// 2. Decode CSV records
	// 3. Encode back to CSV bytes
	// 4. Write to output
	byteStream := io.FromReader(reader, 1024)
	records := csv.DecodeCSV().Apply(byteStream)
	encoded := csv.EncodeCSV().Apply(records)
	result := io.ToWriter(&output).Apply(encoded)

	// Execute the pipeline
	for res := range result.Emit(ctx) {
		if res.IsError() {
			fmt.Printf("Error: %v\n", res.Error())
		}
	}

	fmt.Println("Input:")
	fmt.Println(inputCSV)
	fmt.Println("Output:")
	fmt.Println(output.String())
	fmt.Println()
}

// transformPipeline demonstrates transforming CSV data within the pipeline
func transformPipeline() {
	fmt.Println("--- Transform CSV Pipeline ---")
	ctx := context.Background()

	// Input: Sales data
	inputCSV := `product,quantity,price
Widget,10,9.99
Gadget,5,19.99
Gizmo,8,14.99
`

	reader := strings.NewReader(inputCSV)
	var output bytes.Buffer

	// Build the pipeline with transformation
	byteStream := io.FromReader(reader, 1024)
	records := csv.DecodeCSV().Apply(byteStream)

	// Transform: Add a "total" column (quantity * price)
	transformed := flow.Map(func(record []string) ([]string, error) {
		if len(record) < 3 {
			return record, nil
		}

		// Skip header row
		if record[0] == "product" {
			return append(record, "total"), nil
		}

		qty, _ := strconv.Atoi(record[1])
		price, _ := strconv.ParseFloat(record[2], 64)
		total := float64(qty) * price

		return append(record, fmt.Sprintf("%.2f", total)), nil
	}).Apply(records)

	encoded := csv.EncodeCSV().Apply(transformed)
	result := io.ToWriter(&output).Apply(encoded)

	// Execute the pipeline
	for res := range result.Emit(ctx) {
		if res.IsError() {
			fmt.Printf("Error: %v\n", res.Error())
		}
	}

	fmt.Println("Input:")
	fmt.Println(inputCSV)
	fmt.Println("Output (with total column):")
	fmt.Println(output.String())
	fmt.Println()
}

// filterAndAggregate demonstrates filtering CSV records and computing aggregates
func filterAndAggregate() {
	fmt.Println("--- Filter and Aggregate CSV ---")
	ctx := context.Background()

	// Input: Employee data
	inputCSV := `name,department,salary
Alice,Engineering,85000
Bob,Sales,65000
Charlie,Engineering,95000
Diana,Marketing,70000
Eve,Engineering,90000
`

	reader := strings.NewReader(inputCSV)
	var output bytes.Buffer

	// Build the pipeline
	byteStream := io.FromReader(reader, 1024)
	records := csv.DecodeCSV().Apply(byteStream)

	// Skip header and filter to only Engineering department
	skipped := csv.SkipHeader().Apply(records)
	filtered := filter.Where(func(record []string) bool {
		return len(record) >= 2 && record[1] == "Engineering"
	}).Apply(skipped)

	// Add header back using Concat to prepend
	header := flow.Once([]string{"name", "department", "salary"})
	withHeader := combine.Concat(header, filtered)

	encoded := csv.EncodeCSV().Apply(withHeader)
	result := io.ToWriter(&output).Apply(encoded)

	// Execute and collect for aggregate calculation
	var total int
	var count int
	for res := range result.Emit(ctx) {
		if res.IsError() {
			fmt.Printf("Error: %v\n", res.Error())
			continue
		}
		// Quick parse to compute average (not typical, just for demo)
		line := string(res.Value())
		if strings.Contains(line, ",Engineering,") {
			parts := strings.Split(strings.TrimSpace(line), ",")
			if len(parts) >= 3 {
				if salary, err := strconv.Atoi(parts[2]); err == nil {
					total += salary
					count++
				}
			}
		}
	}

	fmt.Println("Input:")
	fmt.Println(inputCSV)
	fmt.Println("Output (Engineering department only):")
	fmt.Println(output.String())
	if count > 0 {
		fmt.Printf("Average Engineering salary: $%.2f\n", float64(total)/float64(count))
	}
	fmt.Println()
}
