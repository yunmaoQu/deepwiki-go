package main

import (
	"flag"
	"fmt"
	"github.com/deepwiki-go/internal/api"
)

func main() {
	secret := flag.String("secret", "", "JWT secret key")
	flag.Parse()

	if *secret == "" {
		fmt.Println("Error: JWT secret key is required")
		flag.Usage()
		return
	}

	token, err := api.GenerateTestToken(*secret)
	if err != nil {
		fmt.Printf("Error generating token: %v\n", err)
		return
	}

	fmt.Printf("Generated JWT token: %s\n", token)
}
