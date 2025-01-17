package client_test

import (
	"fmt"
	"testing"

	"github.com/lianyun0502/quote_engine/client"
)

func TestQuoteClient(t *testing.T){
	c, err := client.NewQuoteClient("localhost", "7777")
	if err != nil {
		t.Fatal(err)
	}
	quotes, err := c.ListQuotes()
	if err != nil {
		t.Fatal(err)
	}
	if len(quotes) == 0 {
		t.Fatal("no quotes")
	}
	fmt.Println(quotes)
	for _, quote := range quotes {
		t.Log(quote)
	}

}