package main

import (
	"fmt"
	"strings"
)

type Attachment struct {
	name		string
	size		int64
	url			string
}

func (a *Attachment) ToString() string {
	var str strings.Builder
	
	str.WriteString(fmt.Sprintf("Attachment Name: %v\n", a.name))
	str.WriteString(fmt.Sprintf("Size: %d\n", a.size))
	str.WriteString(fmt.Sprintf("URL: %v\n", a.url))
	return str.String()
}