package main

import (
	"fmt"
	"strings"
	"github.com/alexmk92/stringutil"
	"regexp"
	"strconv"
	"sync"
)

/*
 |------------------------------------------------------------------
 | Type: Item
 |------------------------------------------------------------------
 |
 | Represents an item, when we fetch its data we first attempt to
 | hit our file cache, if the item doesn't exist there we fetch
 | it from the Wiki and then store it to our Mongo store
 |
 | @member name (string): Name of the item (url encoded)
 | @member displayName (string): Name of the item (browser friendly)
 | @member imageSrc (string): URL for the image stored on wiki
 | @member price (float32): The advertised price
 | @member statistics ([]Statistic): An array of all stats for this item
 |
 */

type Item struct {
	Name string
	Price float32
	Quantity int32
}

// Public method to fetch data for this item, in Go public method are
// capitalised by convention (doesn't actually enforce Public/Private methods in go)
// this method will call fetchDataFromWiki and fetchDataFromCache where appropriate
func (i *Item) FetchData(wg *sync.WaitGroup, out chan <- Item) {
	fmt.Println("Fetching data for item: ", i.Name)
	i.getQuantityData()
	i.getPricingData()


	if(i.fetchDataFromSQL()) {
		out <- Item{i.Name, i.Price, i.Quantity}
		wg.Done()
	} else {
		i.Save()
		fmt.Println("All saved up")
		out <- Item{i.Name, i.Price, i.Quantity}
		wg.Done()
	}
}

// Extracts quantity data from the item name
func (i *Item) getQuantityData() {
	hasQuantityData, _ := regexp.MatchString("(x ?[0-9]+|[0-9]+ ?x)", i.Name)
	if hasQuantityData {
		isItemStack := false
		if stringutil.CaseInsenstiveContains(i.Name, " stack", " stack ", "stack ") {
			isItemStack = true
			i.Name = strings.TrimSpace(regexp.MustCompile("(?i)(stack[s]?)").ReplaceAllString(i.Name, ""))
			fmt.Println("Name is now: ", i.Name)
		}

		//isPricePerUnit := false
		if stringutil.CaseInsenstiveContains(i.Name, " each", " each ", " per ", " per") {
			//isPricePerUnit = true
			i.Name = strings.TrimSpace(regexp.MustCompile("(?i)(( per[\\s\\n])|( each[\\s\\n])|( ea[\\s\\n]))").ReplaceAllString(i.Name, ""))
			fmt.Println("Name is now: ", i.Name)
		}

		quantityData := regexp.MustCompile("(x ?[0-9]+|[0-9]+ ?x)").FindAllString(i.Name, 1)
		if len(quantityData) > 0 {
			// Replace the quantity data in item name
			i.Name = strings.TrimSpace(regexp.MustCompile("(x ?[0-9]+|[0-9]+ ?x)").ReplaceAllString(i.Name, ""))

			// Remove all non numeric characters so we can get the qty
			fmt.Println("Quantity is: ", quantityData[0])
			quantityData[0] = regexp.MustCompile("[^0-9.]").ReplaceAllString(quantityData[0], "")

			quantity, err := strconv.ParseFloat(quantityData[0], 32)
			if err == nil {
				if isItemStack {
					i.Quantity = int32(quantity * 20)
				} else {
					i.Quantity = int32(quantity)
				}
			} else {
				panic(err)
			}

			fmt.Println("Item is now: ", i)
		}
	} else {
		i.Quantity = 1
	}
}

// Checks if the item has a price associated with its name set
// in the parser stage, if so amend the name to strip the price from
// it and set the price of the item on the struct
func (i *Item) getPricingData() {
	// Check if the price is price per unit or combined price
	hasPricingData, _ := regexp.MatchString("([a-zA-Z ]+)([0-9]+[pkm]?)", i.Name)
	if hasPricingData {
		priceData := regexp.MustCompile("[0-9]+[pkm]?").FindAllString(i.Name, 1)
		if len(priceData) > 0 {
			fmt.Println("Price data is: ", priceData)
			priceIndex := stringutil.CaseInsensitiveIndexOf(i.Name, priceData[len(priceData)-1])
			if(priceIndex > -1) {
				var modifier float32 = 1.0
				// trim and get the last character so we can check the modifier
				compare := strings.ToLower(strings.TrimSpace(priceData[0])[len(priceData[0])-1:len(priceData[0])])
				if compare == "k" {
					modifier = 1000.0
				} else if compare == "m" {
					modifier = 1000000.0
				}
				priceData[0] = regexp.MustCompile("[^0-9.]").ReplaceAllString(priceData[0], "")

				i.Name = TitleCase(strings.TrimSpace(i.Name[0:priceIndex]), false)
				price, err := strconv.ParseFloat(priceData[len(priceData)-1], 32)
				if err == nil {
					i.Price = float32(price) * modifier
				} else {
					panic(err)
				}
			}

			LogInDebugMode("Item is now: ", i)
		}
	}
}

// Check our cache first to see if the item exists - this will eventually return something
// other than a bool, it will return a parsed Item struct from a deserialised JSON object
// sent back from the mongo store
func (i *Item) fetchDataFromSQL() bool {
	var (
		name string
	)

	query := "SELECT name " +
		"FROM items " +
		"WHERE name = ? " +
		"OR displayName = ?"

	rows := DB.Query(query, i.Name, i.Name)
	if rows != nil {
		for rows.Next() {
			err := rows.Scan(&name)
			fmt.Println("NAME IS: ", name)
			if err != nil {
				fmt.Println("Scan error: ", err)
			}
			DB.CloseRows(rows)
			return true
		}
		DB.CloseRows(rows)
	}

	LogInDebugMode("No record found in our SQL database for item: ", i.Name)
	return false
}

// Very basic save functionality to save item to DB, main save will be done
// inside of the Wiki parser
func (i *Item) Save() {
	if i.Name != "" {
		query := "INSERT IGNORE INTO items" +
			"(name, displayName)" +
			"VALUES (?, ?)"

		id, err := DB.Insert(query, TitleCase(i.Name, false), TitleCase(i.Name, true))
		if err != nil {
			fmt.Println(err.Error())
		} else if id == 0 {
			fmt.Println("Item already exists")
		} else if id > 0 {
			fmt.Println("Successfully created item: " + i.Name + " with id: ", id)
		}
	}
}
