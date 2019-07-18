package regext

import (
	"regexp"
	"fmt"
	"testing"
)

func Test_C2(*testing.T) {
	text := `The Rime of the Ancient Mariner`
	a1 := `\b\w{7}\b`
	count := 0
	count++
	fmt.Println("------------  ", count, "   -------------")
	reg, err := regexp.Compile(a1)
	check(err)
	fmt.Println(reg.FindString(text))

}

/*

﻿sed -n 's/^/<h1>;s$/<\/h1>/p;q'rime.txt

*/
