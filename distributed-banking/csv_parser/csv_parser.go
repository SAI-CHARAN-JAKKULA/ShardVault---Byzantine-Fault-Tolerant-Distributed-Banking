package csv_parser

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type Transaction struct {
	Source      int
	Destination int
	Amount      int
}

type Set struct {
	SetNumber           int
	Transactions        []Transaction
	ActiveServerList    []string
	ContactServerList   []string // Additional list for the lab2 test cases
	ByzantineServerList []string
}

func ParseCSV(filename string) ([]Set, error) {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		log.Fatal(err)
	}

	var sets []Set
	var currentSet *Set
	numPattern := regexp.MustCompile(`^-?\d+$`)

	for _, row := range records {
		if len(row) > 0 {
			line := row[0]
			transactionStr := row[1]
			transactionStr = strings.Trim(transactionStr, "()")
			transactionParts := strings.Split(transactionStr, ",")
			source, serr := strconv.Atoi(strings.TrimSpace(transactionParts[0]))
			if serr != nil {
				fmt.Println("Error parsing source:", serr)
				return nil, fmt.Errorf("error parsing source: %v", serr)
			}
			destination, derr := strconv.Atoi(strings.TrimSpace(transactionParts[1]))
			if derr != nil {
				fmt.Println("Error parsing destination:", derr)
				return nil, fmt.Errorf("error parsing destination: %v", derr)
			}
			amount, err := strconv.Atoi(strings.TrimSpace(transactionParts[2]))
			if err != nil {
				fmt.Println("Error parsing amount:", err)
				return nil, fmt.Errorf("error parsing amount: %v", err)
			}
			transaction := Transaction{
				Source:      source,
				Destination: destination,
				Amount:      amount,
			}

			if numPattern.MatchString(line) {
				if currentSet != nil {
					sets = append(sets, *currentSet)
				}
				setNumber, _ := strconv.Atoi(line)
				var activeServerList []string
				setListStr := strings.Trim(row[2], "[]")
				activeServerList = strings.Split(setListStr, ",")
				for i := range activeServerList {
					activeServerList[i] = strings.TrimSpace(activeServerList[i])
				}

				// Extracting the byzantine server list from the row for lab2 test cases
				contactServerList := make([]string, 0)
				byzantineServerList := make([]string, 0) // New list for Byzantine servers
				if len(row) > 3 {
					newListStr := strings.Trim(row[3], "[]")
					if len(newListStr) != 0 {
						contactServerList = strings.Split(newListStr, ",")
						for i := range contactServerList {
							contactServerList[i] = strings.TrimSpace(contactServerList[i])
						}
					}
				}
				// Extracting the byzantine server list from the row
				if len(row) > 4 { // Assuming Byzantine servers are in the 5th column
					byzantineListStr := strings.Trim(row[4], "[]")
					if len(byzantineListStr) != 0 {
						byzantineServerList = strings.Split(byzantineListStr, ",")
						for i := range byzantineServerList {
							byzantineServerList[i] = strings.TrimSpace(byzantineServerList[i])
						}
					}
				}
				currentSet = &Set{
					SetNumber:           setNumber,
					ActiveServerList:    activeServerList,
					ContactServerList:   contactServerList,
					ByzantineServerList: byzantineServerList, // Add Byzantine server list to the Set
					Transactions:        []Transaction{},
				}

				currentSet.Transactions = append(currentSet.Transactions, transaction)

			} else {
				if currentSet != nil {
					currentSet.Transactions = append(currentSet.Transactions, transaction)
				}
			}
		}
	}

	if currentSet != nil {
		sets = append(sets, *currentSet)
	}

	return sets, nil
}
