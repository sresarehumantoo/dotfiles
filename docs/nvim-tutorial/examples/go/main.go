package main

import (
	"fmt"
	"strings"
)

type Item struct {
	Name  string
	Price float64
}

func total(items []Item) float64 {
	sum := 0.0
	for _, it := range items {
		sum += it.Price
	}
	return sum
}

func label(title string, verbose bool) string {
	return strings.ToUpper(title)
}

func main() {
	items := []Item{
		{Name: "tea", Price: 2.5},
		{Name: "bread", Price: 3.25},
	}
	heading := label("receipt", true)
	if strings.Index(heading, "CEIPT") != -1 {
		fmt.Println(heading)
	}
	fmt.Printf("total: %.2f\n", total(items))
}
