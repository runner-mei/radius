package main

import (
	"flag"
	"fmt"
	"log"
	"os"
//	"os/exec"
//	"strings"
//	"unicode"

	"github.com/cuu/radius"
)

var secret = flag.String("secret", "testing123", "shared RADIUS secret between clients and server")
var command string
var arguments []string

func acct_handler(w radius.ResponseWriter, p *radius.Packet) {
	for _, attr := range p.Attributes {
		name, ok := p.Dictionary.Name(attr.Type)
		if !ok{
			continue
		}
		value  := fmt.Sprint(attr.Value)
		log.Printf("%s %s",name,value)
	}

	var attributes []*radius.Attribute
	attributes = []*radius.Attribute{
		p.Dictionary.MustAttr("Reply-Message", "Done"),
	}

	w.AccountingResponse(attributes...)
}

const usage = `
eg:
./acct -secret testing123

`

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [flags] <program> [program arguments...]\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprint(os.Stderr, usage)
	}
	flag.Parse()

	if *secret == "" {
		flag.Usage()
		os.Exit(1)
	}


	log.Println("rad acct server starting")

	acct_server := radius.Server{
		Handler:	 radius.HandlerFunc(acct_handler),
		Secret:		[]byte(*secret),
		Dictionary: radius.Builtin,
		Addr:		":1813",
	}
	if err := acct_server.ListenAndServe(); err != nil{
		log.Fatal(err)
	}

}
