# buildbuildio/pebbles

pebbles is a GraphQL federation gateway.

## How it works
It binds different GraphQL services via Relay Global Object Identification. In order to bind two types which spreads across multiple services, they must implement Node interface, ie
```
# Service A
interface Node {
	id: ID!
}

type Human implements Node {
	id: ID!
	name: String!
}

type Query {
	node(id: ID!): Node
	getHumans: [Human!]!
}

# Service B
interface Node {
	id: ID!
}

type Human implements Node {
	id: ID!
	phone: String!
}

type Animal {
	name: String!
	owner: Human!
}

type Query {
	node(id: ID!): Node
	getAnimals: [Animal!]!
}

# Pebbles resulting Schema
interface Node {
	id: ID!
}

type Human implements Node {
	id: ID!
	name: String!
	phone: String!
}

type Animal {
	name: String!
	owner: Human!
}

type Query {
	node(id: ID!): Node
	getHumans: [Human!]!
	getAnimals: [Animal!]!
}
```

## Why use pebbles?
Our main goals are simple:
1. Work with every valid schema, even with most complicated ones;
2. Provide extendable schemas -- extend types, interfaces, directives and etc without unnecessary duplication;
3. Customize as you grow -- all our building blocks are interfaces, so you can write your own.

## Example
```
package main

import (
	"fmt"
	"net/http"

	"github.com/buildbuildio/pebbles"
)

func main() {
	urls := []string{
		"http://localhost:3000",
	}

	gw, err := pebbles.NewGateway(
		urls,
		pebbles.WithDefaultPlayground(),
	)
	if err != nil {
		panic(err)
	}

	sm := http.NewServeMux()

	sm.HandleFunc("/graphql", gw.Handler)

	server := http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", 8000),
		Handler: sm,
	}

	fmt.Println("Starting server...")
	if err = server.ListenAndServe(); err != nil {
		panic(err)
	}
}
```

## Special thanks
Thanks to [nautilus/gateway](https://github.com/nautilus/gateway) and [movio/bramble](https://github.com/movio/bramble) for inspiration to write this project. Check them, they're both great in their own way ;)