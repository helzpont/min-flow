// Package main demonstrates building an ETL (Extract-Transform-Load) pipeline
// that processes CSV data, transforms it, and outputs JSON.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/aggregate"
	"github.com/lguimbarda/min-flow/flow/csv"
	"github.com/lguimbarda/min-flow/flow/filter"
	"github.com/lguimbarda/min-flow/flow/io"
	flowjson "github.com/lguimbarda/min-flow/flow/json"
)

// Person represents a person record from the CSV
type Person struct {
	Name    string `json:"name"`
	Age     int    `json:"age"`
	City    string `json:"city"`
	Country string `json:"country"`
}

// Statistics represents aggregated statistics
type Statistics struct {
	TotalPeople int     `json:"total_people"`
	AvgAge      float64 `json:"avg_age"`
	Cities      int     `json:"cities"`
}

func main() {
	// Create sample data
	tmpDir, err := os.MkdirTemp("", "etl-example")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	inputFile := filepath.Join(tmpDir, "people.csv")
	outputFile := filepath.Join(tmpDir, "adults.json")

	// Create sample CSV data
	csvData := `name,age,city,country
Alice,30,New York,USA
Bob,17,London,UK
Charlie,45,Paris,France
Diana,22,Tokyo,Japan
Eve,15,Berlin,Germany
Frank,35,Sydney,Australia`

	if err := os.WriteFile(inputFile, []byte(csvData), 0644); err != nil {
		panic(err)
	}

	fmt.Println("=== ETL Pipeline Example ===")
	fmt.Println()

	ctx := context.Background()

	// Step 1: Extract - Read CSV records
	records := csv.ReadRecords(inputFile)

	// Step 2: Skip header row
	dataRows := csv.SkipHeader().Apply(records)

	// Step 3: Transform - Parse CSV records into Person structs
	people := flow.Map(func(record []string) (Person, error) {
		if len(record) != 4 {
			return Person{}, fmt.Errorf("invalid record: expected 4 fields, got %d", len(record))
		}
		age, err := strconv.Atoi(record[1])
		if err != nil {
			return Person{}, fmt.Errorf("invalid age: %s", record[1])
		}
		return Person{
			Name:    record[0],
			Age:     age,
			City:    record[2],
			Country: record[3],
		}, nil
	}).Apply(dataRows)

	// Step 4: Filter - Keep only adults (age >= 18)
	adults := filter.Where(func(p Person) bool {
		return p.Age >= 18
	}).Apply(people)

	// Step 5: Load - Convert to JSON and write to file
	jsonLines := flowjson.Encode[Person]().Apply(adults)

	// Collect JSON lines to write
	output := io.WriteLines(outputFile).Apply(jsonLines)

	// Process the stream
	var count int
	for res := range output.All(ctx) {
		if res.IsError() {
			fmt.Printf("Error: %v\n", res.Error())
		} else {
			count++
		}
	}

	fmt.Printf("Processed %d adult records\n", count)

	// Read and display output
	fmt.Println("\nOutput JSON (adults.json):")
	content, _ := os.ReadFile(outputFile)
	fmt.Println(string(content))

	// Demonstrate aggregation pipeline
	fmt.Println("\n=== Aggregation Pipeline ===")
	aggregationExample(inputFile)
}

// aggregationExample shows how to compute statistics over a stream
func aggregationExample(inputFile string) {
	ctx := context.Background()

	// Read and parse all people
	records := csv.ReadRecords(inputFile)
	dataRows := csv.SkipHeader().Apply(records)

	people := flow.Map(func(record []string) (Person, error) {
		age, _ := strconv.Atoi(record[1])
		return Person{
			Name:    record[0],
			Age:     age,
			City:    record[2],
			Country: record[3],
		}, nil
	}).Apply(dataRows)

	// Use Fold to compute statistics in a single pass
	type accumulator struct {
		total  int
		sumAge int
		cities map[string]bool
	}

	stats := aggregate.Fold(
		accumulator{cities: make(map[string]bool)},
		func(acc accumulator, p Person) accumulator {
			acc.total++
			acc.sumAge += p.Age
			acc.cities[p.City] = true
			return acc
		},
	).Apply(people)

	// Convert to final statistics
	finalStats := flow.Map(func(acc accumulator) (Statistics, error) {
		avgAge := 0.0
		if acc.total > 0 {
			avgAge = float64(acc.sumAge) / float64(acc.total)
		}
		return Statistics{
			TotalPeople: acc.total,
			AvgAge:      avgAge,
			Cities:      len(acc.cities),
		}, nil
	}).Apply(stats)

	// Output statistics
	jsonStats := flowjson.Encode[Statistics]().Apply(finalStats)

	for res := range jsonStats.All(ctx) {
		if res.IsValue() {
			// Pretty print the stats
			fmt.Println("Statistics:", strings.TrimSpace(res.Value()))
		}
	}
}
