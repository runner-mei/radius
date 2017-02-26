package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"unicode"

	"github.com/blind-oracle/go-radius"
)

var secret = flag.String("secret", "testing123", "shared RADIUS secret between clients and server")
var command string
var arguments []string

func acct_handler(w radius.ResponseWriter, p *radius.Packet) {
	
}

func handler(w radius.ResponseWriter, p *radius.Packet) {
	username, password, ok := p.PAP()
	if !ok {
		w.AccessReject()
		return
	}
	log.Printf("%s with %s requesting access (%s #%d)\n", username,password, w.RemoteAddr(), p.Identifier)

	cmd := exec.Command(command, arguments...)

	cmd.Env = os.Environ()
	for _, attr := range p.Attributes {
		name, ok := p.Dictionary.Name(attr.Type)
		if !ok {
			continue
		}
		name = strings.Map(func(r rune) rune {
			if unicode.IsDigit(r) {
				return r
			}
			if unicode.IsLetter(r) {
				if unicode.IsUpper(r) {
					return r
				}
				return unicode.ToUpper(r)
			}
			return '_'
		}, name)
		value := fmt.Sprint(attr.Value)
		cmd.Env = append(cmd.Env, "RADIUS_"+name+"="+value)
	}

	cmd.Env = append(cmd.Env, "RADIUS_USERNAME="+username, "RADIUS_PASSWORD="+password)

	output, _ := cmd.Output()

	var attributes []*radius.Attribute
	if len(output) > 0 {
		attributes = []*radius.Attribute{
			p.Dictionary.MustAttr("Reply-Message", string(output)),
		}
	}

	if cmd.ProcessState.Success() {
		log.Printf("%s accepted (%s #%d)\n", username, w.RemoteAddr(), p.Identifier)
		w.AccessAccept(attributes...)
	} else {
		log.Printf("%s rejected (%s #%d)\n", username, w.RemoteAddr(), p.Identifier)
		w.AccessReject(attributes...)
	}
}

const usage = `
<./auth -secret testing123 echo "fuck you"
>radtest 888 888 localhost 0 testing123
>
>rad_recv: Access-Accept packet from host 127.0.0.1 port 1812, id=2, length=31
>	Reply-Message = "fuck you\n"
or
 ./auth -secret testing123 ./simple-auth
./auth -secret testing123 ./simple-auth 888 888  
in simple-auth, I preset the username and password 

把得到的用户名和密码,变成os.Env中的RADIUS-USERNAMExxxxx 
然后simple-auth 被os.exec,之后对比环境变量,是否是$1 和$2 所指定的用户名密码

`

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [flags] <program> [program arguments...]\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprint(os.Stderr, usage)
	}
	flag.Parse()

	if *secret == "" || flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	command = flag.Arg(0)
	arguments = flag.Args()[1:]

	log.Println("radserver starting")

	server := radius.Server{
		Handler:    radius.HandlerFunc(handler),
		Secret:     []byte(*secret),
		Dictionary: radius.Builtin,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
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
