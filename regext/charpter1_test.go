package regext

import (
	"regexp"
	"fmt"
	"testing"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func Test_C1(t *testing.T) {
	a1 := "707-827-7019"
	b1 := "(100)707-827-7019"
	a2 := "[0-9][0123456789]"
	a3 := "[0-9][0-9][0-9]-[0-9][0-9][0-9]-[0-9][0-9][0-9][0-9]"
	a4 := `\d\d\d-\d\d\d-\d\d\d\d`
	a5 := `\d\d\d\D\d\d\d\D\d\d\d\d`
	//a6 := `(\d)\d\1`
	a7 := `(\d{3,4}[.-]?)+`
	a8 := `(\d{3}-?){2}\d{4}`

	a9 := `^(\(\d{3}\)|\d{3}[.-]?)+\d{3}[.-]?\d{4}$`
	count := 0
	count++
	fmt.Println("------------  ", count, "   -------------")
	reg, err := regexp.Compile(a1)
	check(err)
	fmt.Println(reg.MatchString(b1))
	fmt.Println(reg.FindString(b1))

	count++
	fmt.Println("------------  ", count, "   -------------")
	reg, err = regexp.Compile(a2)
	check(err)
	fmt.Println(reg.MatchString(b1))
	fmt.Println(reg.FindString(b1))

	count++
	fmt.Println("------------  ", count, "   -------------")
	reg, err = regexp.Compile(a3)
	check(err)
	fmt.Println(reg.MatchString(b1))
	fmt.Println(reg.FindString(b1))

	count++
	fmt.Println("------------  ", count, "   -------------")
	reg, err = regexp.Compile(a4)
	check(err)
	fmt.Println(reg.MatchString(b1))
	fmt.Println(reg.FindString(b1))

	count++
	fmt.Println("------------  ", count, "   -------------")
	reg, err = regexp.Compile(a5)
	check(err)
	fmt.Println(reg.MatchString(b1))
	fmt.Println(reg.FindString(b1))

	// failed

	count++
	fmt.Println("------------  ", count, "   -------------")
	//reg = regexp.MustCompile(a6)
	//check(err)
	//fmt.Println(reg.MatchString(b1))
	//fmt.Println(reg.FindString(b1))

	count++
	fmt.Println("------------  ", count, "   -------------")
	reg, err = regexp.Compile(a7)
	check(err)
	fmt.Println(reg.MatchString(b1))
	fmt.Println(reg.FindString(b1))

	count++
	fmt.Println("------------  ", count, "   -------------")
	reg, err = regexp.Compile(a8)
	check(err)
	fmt.Println(reg.MatchString(b1))
	fmt.Println(reg.FindString(b1))

	count++
	fmt.Println("------------  ", count, "   -------------")
	reg, err = regexp.Compile(a9)
	check(err)
	fmt.Println(reg.MatchString(b1))
	fmt.Println(reg.FindString(b1))
}
