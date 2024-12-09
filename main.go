package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/types"
	_ "github.com/mattn/go-sqlite3"
)

var dbPath string
var showLineLabel bool

// generate random graph nodes
func generateGraphNodes(db *sql.DB) []opts.GraphNode {
	rows, err := db.Query(`
		SELECT 
		COALESCE(content->>'name', content->>'address', content->>'cidr', content->>'number', 'No name') AS name_or_address,
		COUNT(r.from_asset_id) + 5 AS relations_count,
		assets.type
	FROM 
		assets
	LEFT JOIN 
		relations r ON assets.id = r.from_asset_id
	GROUP BY 
		assets.id, content->>'name', content->>'address', content->>'cidr', content->>'number';
	`)
	if err != nil {
		log.Fatalf("Failed to get assets")
	}

	nodes := make([]opts.GraphNode, 0)
	for rows.Next() {
		var node opts.GraphNode
		var value float32
		rows.Scan(&node.Name, &value, &node.Category)
		node.SymbolSize = value
		node.Value = value
		nodes = append(nodes, node)
	}

	return nodes
}

func generateGraphLinks(db *sql.DB) []opts.GraphLink {
	links := make([]opts.GraphLink, 0)

	rows, err := db.Query("SELECT from_asset_id - 1, to_asset_id - 1, type FROM relations;")
	if err != nil {
		log.Fatalf("Failed to get assets")
	}

	for rows.Next() {
		var link opts.GraphLink
		var typeName string
		rows.Scan(&link.Source, &link.Target, &typeName)
		if showLineLabel {
			link.Label = &opts.EdgeLabel{
				Show:      opts.Bool(true), // Display the label    // Example formatter template
				Formatter: fmt.Sprintln(typeName),
			}
		}

		links = append(links, link)
	}

	return links
}

func generationCategories(db *sql.DB) []*opts.GraphCategory {
	var categories []*opts.GraphCategory

	rows, err := db.Query(`SELECT type FROM assets;`)
	if err != nil {
		log.Fatalf("Failed to get assets")
	}

	for rows.Next() {
		var name string
		rows.Scan(&name)
		categories = append(categories, &opts.GraphCategory{
			Name: name,
		})
	}

	return categories
}

func httpserver(w http.ResponseWriter, _ *http.Request) {

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to open SQL file: %v", err)
	}

	defer db.Close()

	if err = db.Ping(); err != nil {
		log.Fatalf("Failed to connect to SQL db %v", err)
	}

	graph := charts.NewGraph()

	graph.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{Theme: types.ThemeWesteros, Height: "1000px", Width: "1200px"}),
		charts.WithTitleOpts(opts.Title{
			Title: "DNS Graph by ironicbrew",
		}),
	)

	nodes := generateGraphNodes(db)
	links := generateGraphLinks(db)
	categories := generationCategories(db)

	graph.AddSeries("Series", nodes, links).
		SetSeriesOptions(
			charts.WithGraphChartOpts(opts.GraphChart{Force: &opts.GraphForce{Repulsion: 150}, Draggable: opts.Bool(true), Categories: categories}),
			charts.WithLabelOpts(opts.Label{Show: opts.Bool(true), Position: "right"}),
		)

	graph.PageTitle = "Amass-Viz by ironicbrew"

	// Render the graph chart
	graph.Render(w)

}

func prettyPrint(data interface{}) {
	jsonData, _ := json.MarshalIndent(data, "", "  ")
	fmt.Println(string(jsonData))
}

func main() {

	labels := flag.Bool("labels", false, "Show relation type on lines (Slow performance)")
	path := flag.String("dbPath", "./amass.sqlite", "Path to the database file")
	flag.Parse()

	dbPath = *path
	showLineLabel = *labels

	http.HandleFunc("/", httpserver)
	http.ListenAndServe(":8081", nil)
}
